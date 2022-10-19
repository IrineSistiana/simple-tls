package grpc_lb

import (
	"context"
	"fmt"
	"github.com/IrineSistiana/simple-tls/core/grpc_tunnel"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"net"
	"sync"
	"time"
)

const (
	DefaultMaxStreamPreConn = 4
	CCIdleTimeout           = time.Second * 60
)

type ConnPool struct {
	opts ConnPoolOpts

	m       sync.Mutex
	readyCc map[*grpc.ClientConn]*connStatus
	busyCc  map[*grpc.ClientConn]*connStatus
}

func NewConnPool(opts ConnPoolOpts) *ConnPool {
	opts.init()
	return &ConnPool{
		opts: opts,

		readyCc: make(map[*grpc.ClientConn]*connStatus),
		busyCc:  make(map[*grpc.ClientConn]*connStatus),
	}
}

type ConnPoolOpts struct {
	Target           string
	MaxStreamPerConn int
	ServiceName      string

	// DialOpts for grpc.Dial. Must not have grpc.WithBlock.
	DialOpts []grpc.DialOption

	Logger *zap.Logger
}

func (opts *ConnPoolOpts) init() {
	if opts.MaxStreamPerConn <= 0 {
		opts.MaxStreamPerConn = DefaultMaxStreamPreConn
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
}

type connStatus struct {
	idleTimer     *time.Timer // an idle timer to close balancer.SubConn. May be nil.
	ongoingStream int         // this field will be updated by picker.
}

func (p *ConnPool) GetConn(ctx context.Context) (net.Conn, error) {
	cc, err := p.getCc()
	if err != nil {
		return nil, fmt.Errorf("failed to get client conn, %w", err)
	}
	grpcClient := grpc_tunnel.NewGRPCTunnelClientAddon(cc, p.opts.ServiceName)

	// rpcCtx is a context for the grpc stream.
	rpcCtx, cancel := context.WithCancel(context.Background())
	dialDone := make(chan struct{})

	// This goroutine ensures that ctx can indirectly cancel Connect
	// in its dialing phase.
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-dialDone:
		}
	}()

	stream, err := grpcClient.Connect(rpcCtx)
	close(dialDone)
	if err != nil {
		p.ccStreamDone(cc)
		return nil, err
	}
	return wrapCC(stream, p, cc), nil
}

// getCc retrieves a *grpc.ClientConn from pool or dials a new one.
// The retried *grpc.ClientConn must be released by ccStreamDone.
func (p *ConnPool) getCc() (*grpc.ClientConn, error) {
	p.m.Lock()
	defer p.m.Unlock()

	var pickedCc *grpc.ClientConn
	for cc, status := range p.readyCc {
		if status.ongoingStream == 0 { // an idled cc
			if status.idleTimer != nil {
				if ok := status.idleTimer.Stop(); !ok {
					continue // cc is closing due to idle timeout.
				}
			}
		}
		if cc.GetState() == connectivity.TransientFailure {
			continue // cc is recovering.
		}
		status.ongoingStream++
		if status.ongoingStream >= p.opts.MaxStreamPerConn {
			// move cc to busy group.
			delete(p.readyCc, cc)
			p.busyCc[cc] = status
		}
		pickedCc = cc
		break
	}

	if pickedCc != nil {
		return pickedCc, nil
	}

	// no cc available, dial a new one.
	cc, err := p.dialNewCc()
	if err != nil {
		return nil, fmt.Errorf("failed to dial new grpc client conn, %w", err)
	}
	p.readyCc[cc] = &connStatus{ongoingStream: 1}
	p.opts.Logger.Debug("new cc", zap.Int("ready", len(p.readyCc)), zap.Int("busy", len(p.busyCc)))
	return cc, nil
}

// ccStreamDone release a *grpc.ClientConn retained by getCc.
func (p *ConnPool) ccStreamDone(cc *grpc.ClientConn) {
	p.m.Lock()
	defer p.m.Unlock()

	if status, ok := p.readyCc[cc]; ok {
		p.streamDoneUpdateStatusLocked(cc, status)
	}
	if status, ok := p.busyCc[cc]; ok {
		p.streamDoneUpdateStatusLocked(cc, status)
		if status.ongoingStream < p.opts.MaxStreamPerConn {
			delete(p.busyCc, cc)
			p.readyCc[cc] = status
		}
	}
}

// streamDoneUpdateStatusLocked must be called when p.m is locked.
func (p *ConnPool) streamDoneUpdateStatusLocked(cc *grpc.ClientConn, s *connStatus) {
	s.ongoingStream--
	if s.ongoingStream == 0 {
		if s.idleTimer == nil { // First time idle.
			closeAndRemoveCcFromPool := func() {
				p.m.Lock()
				defer p.m.Unlock()

				_ = cc.Close()
				delete(p.readyCc, cc)
				delete(p.busyCc, cc)
				p.opts.Logger.Debug("cc closed by idle timeout", zap.Int("ready", len(p.readyCc)), zap.Int("busy", len(p.busyCc)))
			}
			s.idleTimer = time.AfterFunc(CCIdleTimeout, closeAndRemoveCcFromPool)
		} else {
			s.idleTimer.Reset(CCIdleTimeout)
		}
	}
}

// dialNewCc dials a new *grpc.ClientConn. This must not be blocked.
func (p *ConnPool) dialNewCc() (*grpc.ClientConn, error) {
	return grpc.Dial(p.opts.Target, p.opts.DialOpts...)
}

// releaseStreamConn automatically releases cc to ConnPool when its
// Close is called.
type releaseStreamConn struct {
	net.Conn
	p  *ConnPool
	cc *grpc.ClientConn

	releaseOnce sync.Once
}

func wrapCC(stream grpc_tunnel.TunnelPeer, p *ConnPool, cc *grpc.ClientConn) net.Conn {
	return &releaseStreamConn{
		Conn: NewGrpcPeerConn(stream),
		p:    p,
		cc:   cc,
	}
}

func (r *releaseStreamConn) Close() error {
	r.releaseOnce.Do(func() {
		r.p.ccStreamDone(r.cc)
	})
	return r.Conn.Close()
}
