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

package core

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"sync"
	"time"
)

const (
	maxPaddingSize uint16 = 0xffff // 64Kb

	paddingIntervalThreshold = time.Millisecond * 10
)

var (
	zeros = make([]byte, maxPaddingSize)
)

type frameType uint8

const (
	headerNil     frameType = 0
	headerData    frameType = 1
	headerPadding frameType = 2
)

type paddingConn struct {
	net.Conn

	paddingInRead  bool
	paddingOnWrite bool

	rl           sync.Mutex
	currentFrame frameType
	frameLeft    uint16

	wl *locker
}

func newPaddingConn(c net.Conn, paddingInRead, paddingOnWrite bool) *paddingConn {
	pc := &paddingConn{
		Conn:           c,
		paddingInRead:  paddingInRead,
		paddingOnWrite: paddingOnWrite,
		wl:             newLocker(),
	}

	return pc
}

func (c *paddingConn) Read(b []byte) (n int, err error) {
	return c.read(b)
}

func (c *paddingConn) Write(b []byte) (n int, err error) {
	if !c.paddingOnWrite {
		return c.Conn.Write(b)
	}
	return c.writeFrame(headerData, b, false)
}

var paddingHeaderPool = sync.Pool{New: func() interface{} { return make([]byte, 3) }}

func (c *paddingConn) readHeader() (t frameType, l uint16, err error) {
	h := paddingHeaderPool.Get().([]byte)
	defer paddingHeaderPool.Put(h)

	_, err = io.ReadFull(c.Conn, h)
	if err != nil {
		return 0, 0, err
	}
	return frameType(h[0]), uint16(h[1])<<8 | uint16(h[2]), nil
}

func (c *paddingConn) read(b []byte) (n int, err error) {
	if !c.paddingInRead {
		return c.Conn.Read(b)
	}

	c.rl.Lock()
	defer c.rl.Unlock()

read:
	if c.currentFrame == headerNil {
		t, l, err := c.readHeader() // new frame
		if err != nil {
			return 0, err
		}
		c.currentFrame = t
		c.frameLeft = l
	}

	switch c.currentFrame {
	case headerData:
		if c.frameLeft == 0 {
			return 0, io.EOF
		}

		n1, err := io.LimitReader(c.Conn, int64(c.frameLeft)).Read(b)
		c.frameLeft -= uint16(n1)
		if c.frameLeft == 0 { // this frame is eof
			c.currentFrame = headerNil    // reset currentFrame
			if err == io.EOF && n1 != 0 { // don't raise this EOF
				err = nil
			}
		}
		return n1, err

	case headerPadding:
		buf := acquireIOBuf()
		n1, err := io.CopyBuffer(ioutil.Discard, io.LimitReader(c.Conn, int64(c.frameLeft)), buf)
		releaseIOBuf(buf)
		c.frameLeft -= uint16(n1)
		if c.frameLeft == 0 { // this frame is eof
			c.currentFrame = headerNil
			if err == io.EOF { // don't raise this EOF
				err = nil
			}
		}
		goto read
	default:
		return 0, fmt.Errorf("unexpect frame type, %d", c.currentFrame)
	}
}

var errPaddingDisabled = errors.New("connection padding opt is disabled")

func (c *paddingConn) writePadding(l uint16) (n int, err error) {
	if !c.paddingOnWrite {
		return 0, errPaddingDisabled
	}
	return c.writeFrame(headerPadding, zeros[:l], false)
}

// tryWritePadding will only write padding data when c is idle.
func (c *paddingConn) tryWritePadding(l uint16) (n int, err error) {
	if c.wl.tryLock() {
		defer c.wl.unlock()
		return c.writeFrame(headerPadding, zeros[:l], true)
	}
	return 0, nil
}

// wBufPool is a 64Kb buffer pool
var wBufPool = sync.Pool{New: func() interface{} { return make([]byte, 0xffff) }}

func (c *paddingConn) writeFrame(t frameType, b []byte, disableLocker bool) (n int, err error) {
	if !disableLocker {
		c.wl.lock()
		defer c.wl.unlock()
	}

	// Note:
	// We try to align this to tls frame. So, the largest frame here is 0xffff - 3 bytes.
	buf := wBufPool.Get().([]byte)
	defer wBufPool.Put(buf)

	for n < len(b) {
		f := copy(buf[3:], b[n:])

		buf[0] = byte(t)
		buf[1] = byte(f >> 8)
		buf[2] = byte(f)

		n2, err := c.Conn.Write(buf[:3+f])

		if n2 > 3 {
			n += n2 - 3 // 3 bytes header
		}
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// randomPaddingSize returns a random num between 4 ~ 16
func randomPaddingSize() uint16 {
	return 4 + uint16(rand.Int31n(12))
}
