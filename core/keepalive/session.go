//     Copyright (C) 2020-2021, IrineSistiana
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

package keepalive

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

var (
	ErrOccupied              = errors.New("connection is occupied")
	ErrIOTimeout             = errors.New("io timeout")
	ErrPingTimeout           = errors.New("ping timeout")
	ErrIdleConnectionTimeout = errors.New("idle connection timeout")
)

const (
	cmdInvalid = iota
	cmdNop
	cmdData
	cmdFin
	cmdPing
)

type Session struct {
	subConn *conn
	opt     *Opt

	m             sync.Mutex // Protect the following statuses.
	onGoingStream *Stream
	waitingFin    int
	closeChan     chan struct{}
	closeErr      error

	// Read loop.
	acceptChan     chan *Stream
	readClosedChan chan struct{}

	// Write loop
	writeLoopChan chan frame

	idleConnTimer *time.Timer
}

type Opt struct {
	AcceptNewConnectionFromPeer bool
	ReadBufSize                 int
	IdleTimeout                 time.Duration
	IdleConnectionTimeout       time.Duration
	PingInterval                time.Duration
	PingTimeout                 time.Duration
}

func (o *Opt) getRBS() int {
	if o.ReadBufSize <= 0 {
		return 16 * 1024
	}
	return o.ReadBufSize
}

func (o *Opt) getPT() time.Duration {
	if o.PingTimeout <= 0 {
		return time.Second * 5
	}
	return o.PingTimeout
}

type frame struct {
	cmd    uint8
	result chan writeResult // This should always be a buffed chan.
	data   []byte           // Optional (cmdData).
}

func newFrame(cmd uint8) frame {
	return frame{cmd: cmd, result: make(chan writeResult, 1)}
}

type writeResult struct {
	n   int
	err error
}

func NewSession(c net.Conn, opt *Opt) *Session {
	if opt == nil {
		opt = &Opt{}
	}
	s := &Session{
		subConn: &conn{
			Conn:        c,
			idleTimeout: opt.IdleTimeout,
		},
		opt: opt,

		acceptChan:     make(chan *Stream),
		readClosedChan: make(chan struct{}),
		writeLoopChan:  make(chan frame),
		closeChan:      make(chan struct{}),
	}

	go s.readLoop()
	go s.writeLoop()

	if opt.IdleConnectionTimeout > 0 {
		s.idleConnTimer = time.AfterFunc(opt.IdleConnectionTimeout, func() {
			s.m.Lock()
			defer s.m.Unlock()
			if s.onGoingStream == nil {
				s.CloseWithErr(ErrIdleConnectionTimeout)
			}
		})
	}

	return s
}

func (s *Session) tryStopIdleTimer() {
	if s.idleConnTimer != nil {
		s.idleConnTimer.Stop()
	}
}

func (s *Session) tryStartIdleTimer() {
	if s.idleConnTimer != nil {
		s.idleConnTimer.Reset(s.opt.IdleConnectionTimeout)
	}
}

func (s *Session) Idle() bool {
	s.m.Lock()
	defer s.m.Unlock()

	return s.onGoingStream == nil
}

func (s *Session) Open() (*Stream, error) {
	s.m.Lock()
	defer s.m.Unlock()

	if isClosedChan(s.closeChan) {
		return nil, s.closeErr
	}

	if s.onGoingStream != nil {
		return nil, ErrOccupied
	}

	stream := newStream(s)
	s.onGoingStream = stream
	s.tryStopIdleTimer()
	return stream, nil
}

func (s *Session) Accept() (*Stream, error) {
	return s.AcceptContext(context.Background())
}

func (s *Session) AcceptContext(ctx context.Context) (*Stream, error) {
	if isClosedChan(s.closeChan) {
		return nil, s.closeErr
	}

	select {
	case stream := <-s.acceptChan:
		s.tryStopIdleTimer()
		return stream, nil
	case <-s.closeChan:
		return nil, s.closeErr
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Session) Close() error {
	return s.CloseWithErr(net.ErrClosed)
}

func (s *Session) CloseWithErr(e error) error {
	if isClosedChan(s.closeChan) {
		return s.subConn.Close()
	}
	s.closeErr = e
	close(s.closeChan)
	return s.subConn.Close()
}

// removeStream must only be called once by each stream.
func (s *Session) removeStream(closedByPeer bool) {
	s.m.Lock()
	defer s.m.Unlock()
	if s.onGoingStream == nil {
		panic("invalid removeStream call")
	}

	s.onGoingStream = nil
	if !closedByPeer {
		s.waitingFin++
	}
	s.tryStartIdleTimer()
}

func (s *Session) readLoop() {
	headerBuf := GetBuf(2)
	defer ReleaseBuf(headerBuf)
	readBuf := GetBuf(s.opt.getRBS())
	defer ReleaseBuf(readBuf)
	defer close(s.readClosedChan)

	for {
		// Read cmd header first.
		if _, err := io.ReadFull(s.subConn, headerBuf[:1]); err != nil {
			s.CloseWithErr(fmt.Errorf("failed to read cmd header, %w", err))
			return
		}
		cmd := headerBuf[0]
		switch cmd {
		case cmdNop:
			continue
		case cmdData:
			if _, err := io.ReadFull(s.subConn, headerBuf); err != nil {
				s.CloseWithErr(fmt.Errorf("failed to read data length header, %w", err))
				return
			}
			length := binary.BigEndian.Uint16(headerBuf)

			if length == 0 {
				s.CloseWithErr(errors.New("zero length data frame"))
				return
			}

			// Previous stream's data. Discard it.
			s.m.Lock()
			waitingFin := s.waitingFin
			s.m.Unlock()
			if waitingFin > 0 {
				if err := discardRead(s.subConn, int(length)); err != nil {
					s.CloseWithErr(fmt.Errorf("failed to read and discard previous stream data, %w", err))
					return
				}
				continue
			}

			// Send data to the stream.
			s.m.Lock()
			stream := s.onGoingStream
			if stream == nil { // New stream.
				if !s.opt.AcceptNewConnectionFromPeer {
					s.CloseWithErr(errors.New("unexpected new connection from peer"))
					return
				}
				stream = newStream(s)
				s.onGoingStream = stream
				s.m.Unlock()

				select {
				case s.acceptChan <- stream: // Notify the Accept() call.
				case <-s.closeChan:
					stream.closedWithErrOnce(s.closeErr, false)
					return
				}
			} else {
				s.m.Unlock()
			}
			remain := int(length)
			for remain > 0 {
				buf := readBuf
				if remain < len(buf) {
					buf = buf[:remain]
				}
				n, err := s.subConn.Read(buf)
				remain -= n
				data := GetBuf(n)
				copy(data, buf)
				if n > 0 {
					select {
					case stream.readBufChan <- data:
					case <-stream.closeChan:
						ReleaseBuf(data)
					case <-s.closeChan:
						ReleaseBuf(data)
						return
					}
				}
				if err != nil {
					s.CloseWithErr(fmt.Errorf("failed to read stream data, %w", err))
					return
				}
			}

			continue
		case cmdFin:
			s.m.Lock()
			if s.waitingFin > 0 { // Previous stream's Fin received.
				s.waitingFin--
				s.m.Unlock()
				continue
			}

			// New Fin, close the current stream.
			stream := s.onGoingStream
			s.m.Unlock()
			if stream != nil {
				stream.closedWithErrOnce(io.EOF, true)
			} else { // Unexpect fin.
				s.CloseWithErr(errors.New("unexpected fin"))
			}
		case cmdPing:
			f := newFrame(cmdNop)
			select {
			case s.writeLoopChan <- f:
			// Don't care about the result.
			case <-s.closeChan:
				return
			}
		default:
			s.CloseWithErr(fmt.Errorf("invalid cmd header [%d]", cmd))
			return
		}
	}
}

func (s *Session) writeLoop() {
	for {
		select {
		case <-s.closeChan:
			return
		case f := <-s.writeLoopChan:
			// If a frame was received by writeLoopChan,
			// a writeResult will always be sent to its result chan.
			var n int
			var err error
			switch f.cmd {
			case cmdData:
				n, err = s.writeData(f.data)
			default:
				n, err = s.subConn.Write([]byte{f.cmd})
			}

			f.result <- writeResult{n: n, err: err} // This is a buffed chan.
			if err != nil {
				s.CloseWithErr(fmt.Errorf("failed to write data, %w", err))
				return
			}
		}
	}
}

func (s *Session) pingLoop() {
	if s.opt.PingInterval < 0 {
		return
	}

	pingTimeout := s.opt.getPT()
	ticker := time.NewTicker(s.opt.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			f := newFrame(cmdPing)
			select {
			case s.writeLoopChan <- f:
				// Don't care about the writeResult. If there is an error,
				// the s.closeChan will be closed anyway.
				timeoutTimer := time.NewTimer(pingTimeout)
				select {
				case now := <-timeoutTimer.C:
					if now.Sub(s.subConn.getLastReadTime()) > pingTimeout {
						s.CloseWithErr(ErrPingTimeout)
						timeoutTimer.Stop()
						return
					}
				case <-s.closeChan:
					timeoutTimer.Stop()
					return
				}
				timeoutTimer.Stop()
			case <-s.closeChan:
				return
			}
		case <-s.closeChan:
			return
		}
	}
}

// discardRead reads and discards l bytes data
func discardRead(r io.Reader, l int) error {
	for l > 0 {
		var buf []byte
		if l < 4096 {
			buf = GetBuf(l)
		} else {
			buf = GetBuf(4096)
		}
		n, err := r.Read(buf)
		ReleaseBuf(buf)
		if err != nil {
			return err
		}
		l -= n
	}
	return nil
}

func (s *Session) writeData(b []byte) (int, error) {
	return writeDataFrameTo(s.subConn, b)
}

func writeDataFrameTo(w io.Writer, b []byte) (int, error) {
	n := 0
	remain := b
	buf := GetBuf(1024)
	defer ReleaseBuf(buf)
	for len(remain) > 0 {
		var batch []byte
		if len(remain) <= 65535 {
			batch = remain
		} else {
			batch = remain[:65535]
		}
		remain = remain[len(batch):]

		buf[0] = cmdData
		binary.BigEndian.PutUint16(buf[1:3], uint16(len(batch)))
		nc := copy(buf[3:], batch)
		na, err := w.Write(buf[:nc+3])
		if na > 3 {
			n += na - 3
		}
		if err != nil {
			return n, err
		}
		batch = batch[nc:]
		if len(batch) > 0 {
			na, err = w.Write(batch)
			n += na
			if err != nil {
				return n, err
			}
		}
	}
	return n, nil
}
