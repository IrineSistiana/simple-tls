package grpc_tunnel

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
)

// addon iface or funcs to generated code.

// TunnelPeer is a abbr for both gRPC client and server.
type TunnelPeer interface {
	Send(*Bytes) error
	Recv() (*Bytes, error)
	Context() context.Context
}

// a GRPCTunnelClient that can set custom service name.
type gRPCTunnelClientAddon struct {
	method string
	cc     grpc.ClientConnInterface
}

func NewGRPCTunnelClientAddon(cc grpc.ClientConnInterface, serviceName string) GRPCTunnelClient {
	return &gRPCTunnelClientAddon{method: fmt.Sprintf("/%s/%s", serviceName, GRPCTunnel_ServiceDesc.Streams[0].StreamName), cc: cc}
}

func (c *gRPCTunnelClientAddon) Connect(ctx context.Context, opts ...grpc.CallOption) (GRPCTunnel_ConnectClient, error) {
	stream, err := c.cc.NewStream(ctx, &GRPCTunnel_ServiceDesc.Streams[0], c.method, opts...)
	if err != nil {
		return nil, err
	}
	x := &gRPCTunnelConnectClient{stream}
	return x, nil
}

func RegisterGRPCTunnelServerAddon(s grpc.ServiceRegistrar, srv GRPCTunnelServer, serviceName string) {
	descCopy := GRPCTunnel_ServiceDesc
	descCopy.ServiceName = serviceName
	s.RegisterService(&descCopy, srv)
}
