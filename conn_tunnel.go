//     Copyright (C) 2020, IrineSistiana
//
//     This file is part of simple-tls.
//
//     simple-tls is free software: you can redistribute it and/or modify
//     it under the terms of the GNU General Public License as published by
//     the Free Software Foundation, either version 3 of the License, or
//     (at your option) any later version.
//
//     simple-tls is distributed in the hope that it will be useful,
//     but WITHOUT ANY WARRANTY; without even the implied warranty of
//     MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//     GNU General Public License for more details.
//
//     You should have received a copy of the GNU General Public License
//     along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

var (
	ioCopybuffPool = &sync.Pool{New: func() interface{} {
		return make([]byte, 16*1024)
	}}
)

func acquireIOBuf() []byte {
	return ioCopybuffPool.Get().([]byte)
}

func releaseIOBuf(b []byte) {
	ioCopybuffPool.Put(b)
}

type tunnelContext struct {
	sync.Mutex
	cancelOnce sync.Once

	done bool
	err  error
}

func (fe *tunnelContext) reportAndCancel(err error) {
	fe.Lock()
	defer fe.Unlock()

	fe.cancelOnce.Do(func() {
		fe.done = true
		fe.err = err
	})
}

func (fe *tunnelContext) getErr() error {
	fe.Lock()
	defer fe.Unlock()

	return fe.err
}

func (fe *tunnelContext) isDone() bool {
	fe.Lock()
	defer fe.Unlock()

	return fe.done
}

func (fe *tunnelContext) setDeadline(c net.Conn, t time.Time) error {
	fe.Lock()
	defer fe.Unlock()

	if fe.done {
		return nil
	}

	return c.SetDeadline(t)
}

// openTunnel opens a tunnel between a and b.
func openTunnel(a, b net.Conn, timeout time.Duration) error {
	tc := tunnelContext{}

	go openOneWayTunnel(a, b, timeout, &tc)
	openOneWayTunnel(b, a, timeout, &tc)

	return tc.getErr()
}

// don not use this func, use openTunnel instead
func openOneWayTunnel(dst, src net.Conn, timeout time.Duration, tc *tunnelContext) {
	buf := acquireIOBuf()

	_, err := copyBuffer(dst, src, buf, timeout, tc)

	// a nil err might be an io.EOF err, which is surpressed by copyBuffer.
	// report a nil err means one conn was closed by peer.
	tc.reportAndCancel(err)

	// let another goroutine break from copy loop
	// tc is canceled, no race.
	src.SetDeadline(time.Now())
	dst.SetDeadline(time.Now())

	releaseIOBuf(buf)
}

func copyBuffer(dst net.Conn, src net.Conn, buf []byte, timeout time.Duration, tc *tunnelContext) (written int64, err error) {

	if len(buf) <= 0 {
		panic("buf size <= 0")
	}

	var lastPadding time.Time

	for {
		tc.setDeadline(src, time.Now().Add(timeout))
		nr, er := src.Read(buf)
		if er != nil {
			return written, err
		}

		if ps, ok := src.(*paddingConn); ok { // if src needs to pad
			if ps.writePaddingEnabled() && time.Since(lastPadding) > paddingIntervalThreshold { // time to pad
				tc.setDeadline(ps, time.Now().Add(timeout))
				_, err := ps.writePadding(defaultGetPaddingSize())
				if err != nil {
					return written, fmt.Errorf("write padding data: %v", err)
				}
				lastPadding = time.Now()
			}
		}

		if nr > 0 {
			tc.setDeadline(dst, time.Now().Add(timeout))
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
