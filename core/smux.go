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

package core

import (
	"fmt"
	"github.com/xtaci/smux"
	"log"
	"net"
	"sync"
)

var muxConfig = &smux.Config{
	Version:           1,
	KeepAliveDisabled: true,
	MaxFrameSize:      16 * 1024,
	MaxReceiveBuffer:  64 * 1024,
	MaxStreamBuffer:   32 * 1024,
}

type muxPool struct {
	dialFunc      func() (c net.Conn, err error)
	maxConcurrent int

	sm   sync.Mutex
	sess map[*smux.Session]struct{}

	dm          sync.Mutex
	dialing     *dialCall
	dialWaiting int
}

func newMuxPool(dialFunc func() (c net.Conn, err error), maxConcurrent int) *muxPool {
	if maxConcurrent < 1 {
		panic(fmt.Sprintf("invalid maxConcurrent: %d", maxConcurrent))
	}
	return &muxPool{dialFunc: dialFunc, maxConcurrent: maxConcurrent, sess: map[*smux.Session]struct{}{}}
}

type dialCall struct {
	done chan struct{} // closed when dial is finished
	s    *smux.Session // only valid after done is closed
	err  error
}

func (m *muxPool) GetStream() (stream *smux.Stream, sess *smux.Session, err error) {
	if stream, sess, ok := m.tryGetStream(); ok {
		return stream, sess, nil
	}
	return m.tryGetStreamFlash()
}

func (m *muxPool) MarkDead(sess *smux.Session) {
	m.sm.Lock()
	defer m.sm.Unlock()
	delete(m.sess, sess)
	sess.Close()
}

func (m *muxPool) tryGetStream() (stream *smux.Stream, sess *smux.Session, ok bool) {
	m.sm.Lock()
	defer m.sm.Unlock()
	for sess := range m.sess {
		if sess.NumStreams() < m.maxConcurrent {
			s, err := sess.OpenStream()
			if err != nil {
				log.Printf("sess err: %v", err)
				sess.Close()
				delete(m.sess, sess)
				continue
			}
			return s, sess, true
		}
	}
	return nil, nil, false
}

func (m *muxPool) tryGetStreamFlash() (stream *smux.Stream, sess *smux.Session, err error) {
	var call *dialCall
	m.dm.Lock()
	if m.dialing == nil || (m.dialing != nil && m.dialWaiting >= m.maxConcurrent) {
		m.dialWaiting = 0
		m.dialing = m.dialSessLocked() // needs a new dial
	} else {
		m.dialWaiting++
	}
	call = m.dialing
	defer m.dm.Unlock()

	<-call.done
	sess = call.s
	err = call.err
	if err != nil {
		return nil, nil, err
	}
	stream, err = sess.OpenStream()
	return stream, sess, err
}

func (m *muxPool) dialSessLocked() (call *dialCall) {
	call = &dialCall{
		done: make(chan struct{}),
	}
	go func() {
		c, err := m.dialFunc()
		if err != nil {
			call.err = err
			close(call.done)
			return
		}

		sess, err := smux.Client(c, muxConfig)
		call.s = sess
		call.err = err
		close(call.done)

		m.sm.Lock()
		m.sess[sess] = struct{}{}
		m.sm.Unlock()

		m.dm.Lock()
		m.dialing = nil
		m.dm.Unlock()
	}()
	return call
}
