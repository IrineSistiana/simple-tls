package ctunnel

import (
	"github.com/IrineSistiana/simple-tls/core/alloc"
	"github.com/IrineSistiana/simple-tls/core/utils"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"
)

type TunnelOpts struct {
	IdleTimout time.Duration
}

func (opts *TunnelOpts) init() {
	utils.SetDefaultNum(&opts.IdleTimout, time.Second*300)
}

// OpenTunnel opens a tunnel between a and b.
// It returns the first err encountered.
// a and b will be closed by OpenTunnel.
func OpenTunnel(a, b net.Conn, opts TunnelOpts) error {
	t := newTunnel(a, b, opts)
	go func() {
		_, err := t.copyBuffer(a, b)
		t.closePeersWithErr(err)
	}()
	go func() {
		_, err := t.copyBuffer(b, a)
		t.closePeersWithErr(err)
	}()
	return t.waitUntilClosed()
}

type tunnel struct {
	a, b net.Conn
	opts TunnelOpts

	closeOnce   sync.Once
	closeNotify chan struct{}
	closeErr    error
}

func newTunnel(a, b net.Conn, opts TunnelOpts) *tunnel {
	return &tunnel{a: a, b: b, opts: opts, closeNotify: make(chan struct{})}
}

func (t *tunnel) closePeersWithErr(err error) {
	t.closeOnce.Do(func() {
		t.a.Close()
		t.b.Close()
		t.closeErr = err
		close(t.closeNotify)
	})
}

func (t *tunnel) openOneWayTunnel(dst, src net.Conn) {
	go func() {
		_, err := t.copyBuffer(dst, src)
		t.closePeersWithErr(err)
	}()
}

func (t *tunnel) waitUntilClosed() error {
	<-t.closeNotify
	return t.closeErr
}

func (t *tunnel) copyBuffer(dst net.Conn, src net.Conn) (written int64, err error) {
	var buf []byte
	defer func() {
		if buf != nil {
			alloc.ReleaseBuf(buf)
		}
	}()
	for {
		if buf != nil {
			alloc.ReleaseBuf(buf)
		}
		buf = alloc.GetBuf(6*1024 + rand.Intn(4*1024)) // random buf size
		src.SetDeadline(time.Now().Add(t.opts.IdleTimout))
		nr, er := src.Read(buf)
		if nr > 0 {
			dst.SetDeadline(time.Now().Add(t.opts.IdleTimout))
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}
