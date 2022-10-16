package core

import (
	"bytes"
	"fmt"
	"github.com/IrineSistiana/simple-tls/core/ctunnel"
	"github.com/IrineSistiana/simple-tls/core/grpc_tunnel"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

const (
	grpcAuthHeader = "simple-tls-auth"
)

type grpcServerHandler struct {
	dst         string
	auth        string
	timeout     time.Duration
	outboundBuf int

	grpc_tunnel.UnimplementedGRPCTunnelServer
}

func newGrpcServerHandler(dst, auth string, timeout time.Duration, outboundBuf int) *grpcServerHandler {
	return &grpcServerHandler{
		dst:         dst,
		auth:        auth,
		timeout:     timeout,
		outboundBuf: outboundBuf,
	}
}

func (g grpcServerHandler) Connect(stream grpc_tunnel.GRPCTunnel_ConnectServer) error {
	if len(g.auth) > 0 {
		md, ok := metadata.FromIncomingContext(stream.Context())
		if !ok {
			return status.Errorf(codes.DataLoss, "failed to get metadata")
		}
		s := md.Get(grpcAuthHeader)
		if len(s) != 1 || s[0] != g.auth {
			return status.Errorf(codes.InvalidArgument, "missing or invalid auth header")
		}
	}

	dstConn, err := net.DialTimeout("tcp", g.dst, time.Second*5)
	if err != nil {
		return fmt.Errorf("failed to connect dst, %w", err)
	}
	defer dstConn.Close()
	applyTCPSocketBuf(dstConn, g.outboundBuf)
	return ctunnel.OpenTunnel(dstConn, newGrpcPeerConn(stream), g.timeout)
}

type grpcPeerRWCWrapper struct {
	stream   grpc_tunnel.TunnelPeer
	peerAddr net.Addr

	rm      sync.Mutex
	readBuf *bytes.Buffer

	readChan     chan []byte
	writeBufChan chan []byte // []bytes on this chan is managed by Allocator.

	readDeadline  pipeDeadline
	writeDeadline pipeDeadline

	closeOnce   sync.Once
	closeNotify chan struct{}
	closeErr    error // closeErr will be set before closeNotify was closed.
}

func newGrpcPeerConn(s grpc_tunnel.TunnelPeer) net.Conn {
	p, ok := peer.FromContext(s.Context())
	var addr net.Addr
	if ok {
		addr = p.Addr
	} else {
		addr = grpcPeerAddrUnavailable{}
	}
	c := &grpcPeerRWCWrapper{
		stream:        s,
		peerAddr:      addr,
		readChan:      make(chan []byte),
		writeBufChan:  make(chan []byte),
		readDeadline:  makePipeDeadline(),
		writeDeadline: makePipeDeadline(),
		closeNotify:   make(chan struct{}),
	}
	go c.readLoop()
	go c.writeLoop()
	return c
}

func (g *grpcPeerRWCWrapper) readLoop() {
	for {
		m, err := g.stream.Recv()
		if err != nil {
			g.closeWithErr(err)
			return
		}
		select {
		case g.readChan <- m.B:
		case <-g.closeNotify:
			return
		}
	}
}

func (g *grpcPeerRWCWrapper) writeLoop() {
	for {
		select {
		case buf := <-g.writeBufChan:
			err := g.stream.Send(&grpc_tunnel.Bytes{B: buf})
			ReleaseBuf(buf)
			if err != nil {
				g.closeWithErr(err)
				return
			}
		case <-g.closeNotify:
			return
		}
	}
}

func (g *grpcPeerRWCWrapper) closeWithErr(err error) {
	g.closeOnce.Do(func() {
		g.closeErr = err
		close(g.closeNotify)
	})
}

func (g *grpcPeerRWCWrapper) Read(p []byte) (n int, err error) {
	g.rm.Lock()
	defer g.rm.Unlock()

	if g.readBuf != nil && g.readBuf.Len() != 0 {
		return g.readBuf.Read(p)
	}

	switch {
	case isClosedChan(g.closeNotify):
		return 0, io.EOF
	case isClosedChan(g.readDeadline.wait()):
		return 0, os.ErrDeadlineExceeded
	}

	select {
	case b := <-g.readChan:
		g.readBuf = bytes.NewBuffer(b)
		return g.readBuf.Read(p)
	case <-g.readDeadline.wait():
		return 0, os.ErrDeadlineExceeded
	case <-g.closeNotify:
		return 0, g.closeErr
	}
}

func (g *grpcPeerRWCWrapper) Write(p []byte) (n int, err error) {
	switch {
	case isClosedChan(g.closeNotify):
		return 0, io.EOF
	case isClosedChan(g.writeDeadline.wait()):
		return 0, os.ErrDeadlineExceeded
	}

	// async write, p cannot be directly used.
	buf := GetBuf(len(p))
	copy(buf, p)

	select {
	case g.writeBufChan <- buf:
		return len(p), nil
	case <-g.writeDeadline.wait():
		return 0, os.ErrDeadlineExceeded
	case <-g.closeNotify:
		return 0, g.closeErr
	}
}

func (g *grpcPeerRWCWrapper) Close() error {
	g.closeWithErr(os.ErrClosed)
	return nil
}

func (g *grpcPeerRWCWrapper) LocalAddr() net.Addr {
	return grpcPeerAddrUnavailable{}
}

func (g *grpcPeerRWCWrapper) RemoteAddr() net.Addr {
	return g.peerAddr
}

func (g *grpcPeerRWCWrapper) SetDeadline(t time.Time) error {
	g.readDeadline.set(t)
	g.writeDeadline.set(t)
	return nil
}

func (g *grpcPeerRWCWrapper) SetReadDeadline(t time.Time) error {
	g.readDeadline.set(t)
	return nil
}

func (g *grpcPeerRWCWrapper) SetWriteDeadline(t time.Time) error {
	g.writeDeadline.set(t)
	return nil
}

type grpcPeerAddrUnavailable struct{}

func (g grpcPeerAddrUnavailable) Network() string {
	return "grpc"
}

func (g grpcPeerAddrUnavailable) String() string {
	return "unavailable"
}
