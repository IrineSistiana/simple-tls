//     Copyright (C) 2020, IrineSistiana
//
//     This file is part of simple-tls.
//
//     mosdns is free software: you can redistribute it and/or modify
//     it under the terms of the GNU General Public License as published by
//     the Free Software Foundation, either version 3 of the License, or
//     (at your option) any later version.
//
//     mosdns is distributed in the hope that it will be useful,
//     but WITHOUT ANY WARRANTY; without even the implied warranty of
//     MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//     GNU General Public License for more details.
//
//     You should have received a copy of the GNU General Public License
//     along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
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

type firstErr struct {
	reportrOnce sync.Once
	err         atomic.Value
}

func (fe *firstErr) report(err error) {
	fe.reportrOnce.Do(func() {
		// atomic.Value can't store nil value
		if err != nil {
			fe.err.Store(err)
		}
	})
}

func (fe *firstErr) getErr() error {
	v := fe.err.Load()
	if err, ok := v.(error); ok {
		return err
	}
	return nil
}

// openTunnel opens a tunnel between a and b, if any end
// reports an error during io.Copy, openTunnel will close
// both of them.
func openTunnel(a, b net.Conn, timeout time.Duration) error {
	fe := firstErr{}

	go openOneWayTunnel(a, b, timeout, &fe)
	openOneWayTunnel(b, a, timeout, &fe)

	return fe.getErr()
}

// don not use this func, use openTunnel instead
func openOneWayTunnel(dst, src net.Conn, timeout time.Duration, fe *firstErr) {
	buf := acquireIOBuf()

	_, err := copyBuffer(dst, src, buf, timeout)

	// a nil err might be an io.EOF err, which is surpressed by copyBuffer.
	// report a nil err means one conn was closed by peer.
	fe.report(err)

	//let another goroutine break from copy loop
	src.Close()
	dst.Close()

	releaseIOBuf(buf)
}

func copyBuffer(dst net.Conn, src net.Conn, buf []byte, timeout time.Duration) (written int64, err error) {

	if len(buf) <= 0 {
		panic("buf size <= 0")
	}

	ddlLastSet := time.Time{}

	for {
		if time.Since(ddlLastSet) > time.Millisecond*500 {
			now := time.Now()
			ddlLastSet = now
			src.SetDeadline(now.Add(timeout))
		}
		nr, er := src.Read(buf)
		if nr > 0 {
			if time.Since(ddlLastSet) > time.Millisecond*500 {
				now := time.Now()
				ddlLastSet = now
				dst.SetDeadline(now.Add(timeout))
			}
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
