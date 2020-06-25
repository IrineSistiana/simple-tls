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

	wl   sync.Mutex
	wBuf [3 + 0xffff]byte
}

func newPaddingConn(c net.Conn, paddingInRead, paddingOnWrite bool) *paddingConn {
	return &paddingConn{Conn: c, paddingInRead: paddingInRead, paddingOnWrite: paddingOnWrite}
}

func (c *paddingConn) writePaddingEnabled() bool {
	return c.paddingOnWrite
}

func (c *paddingConn) Read(b []byte) (n int, err error) {
	if !c.paddingInRead {
		return c.Conn.Read(b)
	}

	c.rl.Lock()
	defer c.rl.Unlock()

read:
	if c.currentFrame == headerNil {
		t, l, err := c.readHeader() // new frame
		if err != nil {
			return 0, fmt.Errorf("failed to read header, %w", err)
		}
		c.currentFrame = t
		c.frameLeft = l
	}

	switch c.currentFrame {
	case headerData:
		n1, err := io.LimitReader(c.Conn, int64(c.frameLeft)).Read(b)
		c.frameLeft -= uint16(n1)
		if c.frameLeft == 0 { // this frame is eof
			c.currentFrame = headerNil // reset currentFrame
			if err == io.EOF {         // don't raise this EOF
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

func (c *paddingConn) readHeader() (t frameType, l uint16, err error) {
	h := make([]byte, 3)
	_, err = io.ReadFull(c.Conn, h)
	if err != nil {
		return 0, 0, err
	}
	return frameType(h[0]), uint16(h[1])<<8 | uint16(h[2]), nil
}

func (c *paddingConn) Write(b []byte) (n int, err error) {
	if !c.paddingOnWrite {
		return c.Conn.Write(b)
	}

	c.wl.Lock()
	defer c.wl.Unlock()

	l := len(b)
	for n < l {
		var f int
		dataLeft := l - n
		if dataLeft <= 0xffff {
			f = dataLeft
		} else {
			f = 0xffff
		}

		c.wBuf[0] = byte(headerData)
		c.wBuf[1] = byte(f >> 8)
		c.wBuf[2] = byte(f)

		n1 := copy(c.wBuf[3:], b[n:n+f])
		n2, err := c.Conn.Write(c.wBuf[:3+n1])
		n += n2 - 3 // 3 bytes header
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

var errPaddingDisabled = errors.New("connect padding opt is disabled")

func (c *paddingConn) writePadding(l uint16) (n int, err error) {
	if !c.paddingOnWrite {
		return 0, errPaddingDisabled
	}

	c.wl.Lock()
	defer c.wl.Unlock()

	c.wBuf[0] = byte(headerPadding)
	c.wBuf[1] = byte(l >> 8)
	c.wBuf[2] = byte(l)

	n1 := copy(c.wBuf[3:], zeros[:l])
	return c.Conn.Write(c.wBuf[:3+n1])
}

type paddingBoundConn struct {
	net.Conn
	pc *paddingConn

	rl       sync.Mutex
	lastRead time.Time
}

// defaultGetPaddingSize returns a random num between 4 ~ 16
func defaultGetPaddingSize() uint16 {
	return 4 + uint16(rand.Int31n(12))
}

func boundPaddingConn(c net.Conn, pc *paddingConn) *paddingBoundConn {
	return &paddingBoundConn{Conn: c, pc: pc}
}

func (c *paddingBoundConn) Read(b []byte) (n int, err error) {
	c.rl.Lock()
	defer c.rl.Unlock()

	n, err = c.Conn.Read(b)

	if n > 0 && time.Since(c.lastRead) > paddingIntervalThreshold {
		c.pc.writePadding(defaultGetPaddingSize())
	}
	return n, err
}
