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
	"net"
	"sync"
	"time"
)

type Stream struct {
	sess *Session

	rl        sync.Mutex // Read lock.
	bufHead   []byte     // will be released to pool
	bufUnread []byte

	readBufChan chan []byte

	closeMu   sync.Mutex
	closeChan chan struct{}
	closeErr  error

	// deadlines
	readDeadline  *streamDeadline
	writeDeadline *streamDeadline
}

func newStream(s *Session) *Stream {
	return &Stream{
		sess:          s,
		readBufChan:   make(chan []byte, 1), // A buffed chan makes read faster.
		closeChan:     make(chan struct{}),
		readDeadline:  newSteamDeadline(),
		writeDeadline: newSteamDeadline(),
	}
}

func (s *Stream) Read(b []byte) (int, error) {
	s.rl.Lock()
	defer s.rl.Unlock()

	// Read existing buf.
	if len(s.bufUnread) > 0 {
		return s.readFromBufLocked(b)
	}

	ddl := s.readDeadline.wait()
	var err error
	select {
	case data := <-s.readBufChan:
		s.bufHead = data
		s.bufUnread = data
		return s.readFromBufLocked(b)
	case <-ddl:
		err = ErrIOTimeout
	case <-s.sess.closeChan:
		err = s.sess.closeErr
	case <-s.closeChan:
		err = s.closeErr
	}

	// Make sure the readBufChan is empty, because when they both are ready,
	// some data may be discarded unexpectedly.
	if len(s.readBufChan) > 0 {
		data := <-s.readBufChan
		s.bufHead = data
		s.bufUnread = data
		return s.readFromBufLocked(b)
	}
	return 0, err
}

func (s *Stream) readFromBufLocked(b []byte) (int, error) {
	n := copy(b, s.bufUnread)
	s.bufUnread = s.bufUnread[n:]

	// Release the buf if it is empty.
	if len(s.bufUnread) == 0 && s.bufHead != nil {
		ReleaseBuf(s.bufHead)
		s.bufHead = nil
	}
	return n, nil
}

func (s *Stream) Write(b []byte) (n int, err error) {
	ddl := s.writeDeadline.wait()
	switch {
	case isClosedChan(ddl):
		return 0, ErrIOTimeout
	case isClosedChan(s.sess.closeChan):
		return 0, s.sess.closeErr
	case isClosedChan(s.closeChan):
		return 0, s.closeErr
	}

	f := newFrame(cmdData)
	f.data = b
	select {
	case s.sess.writeLoopChan <- f:
		res := <-f.result
		return res.n, res.err
	case <-ddl:
		return 0, ErrIOTimeout
	case <-s.sess.closeChan:
		return 0, s.sess.closeErr
	case <-s.closeChan:
		return 0, s.closeErr
	}
}

// Close closes the stream. It will never return an error.
func (s *Stream) Close() error {
	s.closedWithErrOnce(net.ErrClosed, false)
	return nil
}

func (s *Stream) closedWithErrOnce(e error, closedByPeer bool) {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if isClosedChan(s.closeChan) {
		return
	}

	s.sess.removeStream(closedByPeer)
	s.closeErr = e
	close(s.closeChan)

	f := newFrame(cmdFin)
	select {
	case s.sess.writeLoopChan <- f:
		<-f.result
		// If there is a write error, the session will be closed anyway.
	case <-s.sess.closeChan:
		// Session is closed
	}
}

func (s *Stream) LocalAddr() net.Addr {
	return s.sess.subConn.LocalAddr()
}

func (s *Stream) RemoteAddr() net.Addr {
	return s.sess.subConn.RemoteAddr()
}

func (s *Stream) SetDeadline(t time.Time) error {
	s.readDeadline.set(t)
	s.writeDeadline.set(t)
	return nil
}

func (s *Stream) SetReadDeadline(t time.Time) error {
	s.readDeadline.set(t)
	return nil
}

func (s *Stream) SetWriteDeadline(t time.Time) error {
	s.writeDeadline.set(t)
	return nil
}

// streamDeadline is copied from golang net.pipeDeadline.
type streamDeadline struct {
	mu     sync.Mutex // Guards timer and cancel
	timer  *time.Timer
	cancel chan struct{} // Must be non-nil
}

func newSteamDeadline() *streamDeadline {
	return &streamDeadline{cancel: make(chan struct{})}

}

func (d *streamDeadline) set(t time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil && !d.timer.Stop() {
		<-d.cancel // Wait for the timer callback to finish and close cancel
	}
	d.timer = nil

	// Time is zero, then there is no deadline.
	closed := isClosedChan(d.cancel)
	if t.IsZero() {
		if closed {
			d.cancel = make(chan struct{})
		}
		return
	}

	// Time in the future, setup a timer to cancel in the future.
	if dur := time.Until(t); dur > 0 {
		if closed {
			d.cancel = make(chan struct{})
		}
		d.timer = time.AfterFunc(dur, func() {
			close(d.cancel)
		})
		return
	}

	// Time in the past, so close immediately.
	if !closed {
		close(d.cancel)
	}
}

func isClosedChan(c <-chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

func (d *streamDeadline) wait() chan struct{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cancel
}
