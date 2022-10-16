package grpc_tunnel

import "context"

type TunnelPeer interface {
	Send(*Bytes) error
	Recv() (*Bytes, error)
	Context() context.Context
}
