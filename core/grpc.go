package core

import (
	"github.com/IrineSistiana/simple-tls/core/grpc_lb"
	"github.com/IrineSistiana/simple-tls/core/grpc_tunnel"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcServerHandler struct {
	connHandler TransportHandler

	grpc_tunnel.UnimplementedGRPCTunnelServer
}

func newGrpcServerHandler(connHandler TransportHandler) *grpcServerHandler {
	return &grpcServerHandler{
		connHandler: connHandler,
	}
}

func (g grpcServerHandler) Connect(stream grpc_tunnel.GRPCTunnel_ConnectServer) error {
	err := g.connHandler.Handle(grpc_lb.NewGrpcPeerConn(stream))
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}
