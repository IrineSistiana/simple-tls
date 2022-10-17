package core

import (
	"fmt"
	"github.com/IrineSistiana/simple-tls/core/ctunnel"
	"github.com/IrineSistiana/simple-tls/core/grpc_lb"
	"github.com/IrineSistiana/simple-tls/core/grpc_tunnel"
	"net"
	"time"
)

type grpcServerHandler struct {
	dst         string
	timeout     time.Duration
	outboundBuf int

	grpc_tunnel.UnimplementedGRPCTunnelServer
}

func newGrpcServerHandler(dst string, timeout time.Duration, outboundBuf int) *grpcServerHandler {
	return &grpcServerHandler{
		dst:         dst,
		timeout:     timeout,
		outboundBuf: outboundBuf,
	}
}

func (g grpcServerHandler) Connect(stream grpc_tunnel.GRPCTunnel_ConnectServer) error {
	dstConn, err := net.DialTimeout("tcp", g.dst, time.Second*5)
	if err != nil {
		return fmt.Errorf("failed to connect dst, %w", err)
	}
	defer dstConn.Close()
	applyTCPSocketBuf(dstConn, g.outboundBuf)
	return ctunnel.OpenTunnel(dstConn, grpc_lb.NewGrpcPeerConn(stream), ctunnel.TunnelOpts{IdleTimout: g.timeout})
}
