package grpc_lb

import (
	"bytes"
	"github.com/IrineSistiana/simple-tls/core/alloc"
	"github.com/IrineSistiana/simple-tls/core/grpc_tunnel"
	"google.golang.org/grpc/peer"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

type GrpcPeerRWCWrapper struct {
	stream   grpc_tunnel.TunnelPeer
	peerAddr net.Addr

	rm      sync.Mutex
	readBuf *bytes.Buffer

	readChan     chan []byte
	writeBufChan chan writeCmd

	readDeadline  pipeDeadline
	writeDeadline pipeDeadline

	closeOnce   sync.Once
	closeNotify chan struct{}
	closeErr    error // closeErr will be set before closeNotify was closed.
}

type writeCmd struct {
	buf []byte     // buf is managed by Allocator.
	err chan error // a chan to receive grpc Send() result.
}

func NewGrpcPeerConn(s grpc_tunnel.TunnelPeer) net.Conn {
	p, ok := peer.FromContext(s.Context())
	var addr net.Addr
	if ok {
		addr = p.Addr
	} else {
		addr = grpcPeerAddrUnavailable{}
	}
	c := &GrpcPeerRWCWrapper{
		stream:        s,
		peerAddr:      addr,
		readChan:      make(chan []byte),
		writeBufChan:  make(chan writeCmd),
		readDeadline:  makePipeDeadline(),
		writeDeadline: makePipeDeadline(),
		closeNotify:   make(chan struct{}),
	}
	go c.readLoop()
	go c.writeLoop()
	return c
}

func (g *GrpcPeerRWCWrapper) readLoop() {
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

func (g *GrpcPeerRWCWrapper) writeLoop() {
	msg := &grpc_tunnel.Bytes{}
	for {
		select {
		case cmd := <-g.writeBufChan:
			msg.B = cmd.buf
			err := g.stream.Send(msg)
			alloc.ReleaseBuf(cmd.buf)
			cmd.err <- err
			if err != nil {
				g.closeWithErr(err)
				return
			}
		case <-g.closeNotify:
			return
		}
	}
}

func (g *GrpcPeerRWCWrapper) closeWithErr(err error) {
	g.closeOnce.Do(func() {
		g.closeErr = err
		close(g.closeNotify)
	})
}

func (g *GrpcPeerRWCWrapper) Read(p []byte) (n int, err error) {
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

func (g *GrpcPeerRWCWrapper) Write(p []byte) (n int, err error) {
	switch {
	case isClosedChan(g.closeNotify):
		return 0, io.EOF
	case isClosedChan(g.writeDeadline.wait()):
		return 0, os.ErrDeadlineExceeded
	}

	// async write, p cannot be directly used.
	buf := alloc.GetBuf(len(p))
	copy(buf, p)

	cmd := writeCmd{
		buf: buf,
		err: make(chan error),
	}
	select {
	case g.writeBufChan <- cmd:
		err := <-cmd.err
		if err != nil {
			return 0, err
		}
		return len(p), nil
	case <-g.writeDeadline.wait():
		return 0, os.ErrDeadlineExceeded
	case <-g.closeNotify:
		return 0, g.closeErr
	}
}

func (g *GrpcPeerRWCWrapper) Close() error {
	g.closeWithErr(os.ErrClosed)
	return nil
}

func (g *GrpcPeerRWCWrapper) LocalAddr() net.Addr {
	return grpcPeerAddrUnavailable{}
}

func (g *GrpcPeerRWCWrapper) RemoteAddr() net.Addr {
	return g.peerAddr
}

func (g *GrpcPeerRWCWrapper) SetDeadline(t time.Time) error {
	g.readDeadline.set(t)
	g.writeDeadline.set(t)
	return nil
}

func (g *GrpcPeerRWCWrapper) SetReadDeadline(t time.Time) error {
	g.readDeadline.set(t)
	return nil
}

func (g *GrpcPeerRWCWrapper) SetWriteDeadline(t time.Time) error {
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
