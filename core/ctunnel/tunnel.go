package ctunnel

import (
	"io"
	"net"
	"sync"
	"time"
)

// OpenTunnel opens a tunnel between a and b.
// It returns the first err encountered.
// a and b will be closed by OpenTunnel.
func OpenTunnel(a, b net.Conn, timeout time.Duration) error {
	t := newTunnel(a, b, timeout)
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
	a, b    net.Conn
	timeout time.Duration

	closeOnce   sync.Once
	closeNotify chan struct{}
	closeErr    error
}

func newTunnel(a, b net.Conn, timeout time.Duration) *tunnel {
	return &tunnel{a: a, b: b, timeout: timeout, closeNotify: make(chan struct{})}
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
	buf := acquireIOBuf()
	defer releaseIOBuf(buf)

	for {
		src.SetDeadline(time.Now().Add(t.timeout))
		nr, er := src.Read(buf)
		if nr > 0 {
			dst.SetDeadline(time.Now().Add(t.timeout))
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

var (
	ioCopyBuffPool = &sync.Pool{New: func() interface{} {
		return make([]byte, 8*1024)
	}}
)

func acquireIOBuf() []byte {
	return ioCopyBuffPool.Get().([]byte)
}

func releaseIOBuf(b []byte) {
	ioCopyBuffPool.Put(b)
}
