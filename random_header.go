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
	"math/rand"
	"net"
	"sync"
	"time"
)

var (
	errRandomHeaderSize = errors.New("random header is too short or too large")
)

const (
	minHeaderSize = 512       // 512b
	maxHeaderSize = 16 * 1024 // 16Kb

	headerSizeWindows = maxHeaderSize - minHeaderSize

	expiredAfter = time.Minute * 30
)

var randomHeaderPool = sync.Pool{
	New: func() interface{} {
		rh := new(randomHeader)
		rand.Read(rh.buf[:])
		rh.expireTime = time.Now().Add(expiredAfter)
		return rh
	},
}

var randomHeaderPoolReadBuf = sync.Pool{
	New: func() interface{} {
		return new(randomHeader)
	},
}

type randomHeader struct {
	buf        [2 + maxHeaderSize]byte
	expireTime time.Time
}

func getRandomHeader() *randomHeader {
	rh := randomHeaderPool.Get().(*randomHeader)
	if time.Now().After(rh.expireTime) {
		rand.Read(rh.buf[:])
		rh.expireTime = time.Now().Add(expiredAfter)
	}
	return rh
}

func releaseRandomHeader(rh *randomHeader) {
	randomHeaderPool.Put(rh)
}

func getRandomHeaderAsReadBuf() *randomHeader {
	rh := randomHeaderPoolReadBuf.Get().(*randomHeader)
	return rh
}

func releaseRandomHeaderAsReadBuf(rh *randomHeader) {
	randomHeaderPoolReadBuf.Put(rh)
}

func getRandomHeaderSize() int {
	return minHeaderSize + rand.Intn(headerSizeWindows)
}

type randomHeaderConn struct {
	net.Conn
	readDone, writeDone bool
}

func (c *randomHeaderConn) Read(b []byte) (n int, err error) {
	if !c.readDone {
		err := readRandomHeaderFrom(c.Conn)
		if err != nil {
			return 0, err
		}
		c.readDone = true
	}
	return c.Conn.Read(b)
}

func (c *randomHeaderConn) Write(b []byte) (n int, err error) {
	if !c.writeDone {
		err := writeRandomHeaderToWithExtraData(c.Conn, b)
		if err != nil {
			return 0, err
		}
		c.writeDone = true
		return len(b), nil
	}
	return c.Conn.Write(b)
}

func readRandomHeaderFrom(c net.Conn) (err error) {
	rh := getRandomHeaderAsReadBuf()
	defer releaseRandomHeaderAsReadBuf(rh)

	if _, err := io.ReadFull(c, rh.buf[:2]); err != nil {
		return fmt.Errorf("failed to read random header size: %w", err)
	}

	headerSize := uint16(rh.buf[1]) | uint16(rh.buf[0])<<8
	if headerSize < minHeaderSize || headerSize > maxHeaderSize {
		return errRandomHeaderSize
	}

	if _, err := io.ReadFull(c, rh.buf[2:2+headerSize]); err != nil {
		return fmt.Errorf("failed to read random header: %w", err)
	}
	return nil
}

func writeRandomHeaderTo(c net.Conn) (err error) {
	return writeRandomHeaderToWithExtraData(c, nil)
}

func writeRandomHeaderToWithExtraData(c net.Conn, data []byte) (err error) {
	rh := getRandomHeader()
	defer releaseRandomHeader(rh)

	headerSize := getRandomHeaderSize()
	rh.buf[0] = byte(headerSize >> 8)
	rh.buf[1] = byte(headerSize)
	if len(data) == 0 {
		_, err = c.Write(rh.buf[:headerSize+2])
	} else {
		buf := make([]byte, 0)
		buf = append(buf, rh.buf[:headerSize+2]...)
		buf = append(buf, data...)
		_, err = c.Write(buf)
	}
	return err
}
