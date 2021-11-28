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
	"context"
	"fmt"
	"github.com/xtaci/smux"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

const (
	modePlain byte = iota
	modeMux
)

func getMuxConf(idleTimeout time.Duration) *smux.Config {
	return &smux.Config{
		Version:           1,
		KeepAliveDisabled: true,
		MaxFrameSize:      16 * 1024,
		MaxReceiveBuffer:  64 * 1024,
		MaxStreamBuffer:   32 * 1024,
		IdleTimeout:       idleTimeout,
	}
}

type MuxTransport struct {
	nextTransport Transport
	maxConcurrent int
	muxConfig     *smux.Config

	sm   sync.Mutex
	sess map[*smux.Session]struct{}

	dm          sync.Mutex
	dialing     *dialCall
	dialWaiting int
}

func NewMuxTransport(subTransport Transport, maxConcurrent int, idleTimeout time.Duration) *MuxTransport {
	return &MuxTransport{
		nextTransport: subTransport,
		maxConcurrent: maxConcurrent,
		muxConfig:     getMuxConf(idleTimeout),
		sess:          map[*smux.Session]struct{}{},
	}
}

type dialCall struct {
	done chan struct{} // closed when dial is finished
	s    *smux.Session // only valid after done is closed
	err  error
}

func (m *MuxTransport) Dial(ctx context.Context) (net.Conn, error) {
	if m.maxConcurrent <= 1 {
		conn, err := m.nextTransport.Dial(ctx)
		if err != nil {
			return nil, err
		}
		if _, err := conn.Write([]byte{modePlain}); err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to write mux header: %w", err)
		}
	}

	if stream := m.tryGetStream(); stream != nil {
		return stream, nil
	}
	return m.tryGetStreamFlash(ctx)
}

func (m *MuxTransport) MarkDead(sess *smux.Session) {
	m.sm.Lock()
	defer m.sm.Unlock()
	delete(m.sess, sess)
	sess.Close()
}

func (m *MuxTransport) tryGetStream() (stream *smux.Stream) {
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
			return s
		}
	}
	return nil
}

func (m *MuxTransport) tryGetStreamFlash(ctx context.Context) (*smux.Stream, error) {
	var call *dialCall
	m.dm.Lock()
	if m.dialing == nil || (m.dialing != nil && m.dialWaiting >= m.maxConcurrent) {
		m.dialWaiting = 0
		m.dialing = m.dialSessLocked(ctx) // needs a new dial
	} else {
		m.dialWaiting++
	}
	call = m.dialing
	defer m.dm.Unlock()

	<-call.done
	sess := call.s
	err := call.err
	if err != nil {
		return nil, err
	}
	return sess.OpenStream()
}

func (m *MuxTransport) dialSessLocked(ctx context.Context) (call *dialCall) {
	call = &dialCall{
		done: make(chan struct{}),
	}
	go func() {
		c, err := m.nextTransport.Dial(ctx)
		if err != nil {
			call.err = err
			close(call.done)
			return
		}

		if _, err := c.Write([]byte{modeMux}); err != nil {
			c.Close()
			call.err = fmt.Errorf("failed to write mux header: %w", err)
			close(call.done)
			return
		}

		sess, err := smux.Client(c, m.muxConfig)
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

type MuxTransportHandler struct {
	nextHandler TransportHandler
	muxConfig   *smux.Config
}

func NewMuxTransportHandler(nextHandler TransportHandler, idleTimeout time.Duration) *MuxTransportHandler {
	return &MuxTransportHandler{nextHandler: nextHandler, muxConfig: getMuxConf(idleTimeout)}
}

func (h *MuxTransportHandler) Handle(conn net.Conn) error {
	header := make([]byte, 1)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("failed to read mux header: %w", err)
	}

	switch header[0] {
	case modePlain:
		return h.nextHandler.Handle(conn)
	case modeMux:
		sess, err := smux.Server(conn, h.muxConfig)
		if err != nil {
			return err
		}
		defer sess.Close()

		for {
			stream, err := sess.AcceptStream()
			if err != nil {
				return nil // suppress smux err
			}
			go func() {
				defer stream.Close()
				if err := h.nextHandler.Handle(stream); err != nil {
					logConnErr(stream, err)
					return
				}
			}()
		}
	default:
		return fmt.Errorf("invalid mux header %d", header[0])
	}
}
