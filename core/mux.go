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
	"github.com/IrineSistiana/simple-tls/core/keepalive"
	"github.com/xtaci/smux"
	"io"
	"net"
	"sync"
	"time"
)

const (
	modePlain byte = iota
	modeSMux
	modeKeepalive
)

func getSMuxConf(idleTimeout time.Duration) *smux.Config {
	return &smux.Config{
		Version:           1,
		KeepAliveDisabled: false,
		KeepAliveInterval: time.Second * 15,
		KeepAliveTimeout:  time.Second * 20,
		MaxFrameSize:      16 * 1024,
		MaxReceiveBuffer:  64 * 1024,
		MaxStreamBuffer:   32 * 1024,
		IdleTimeout:       idleTimeout,
	}
}

type MuxTransport struct {
	nextTransport Transport

	keepalivePool *keepalive.SessPool
	sMuxPool      *sMuxPool
}

type sMuxPool struct {
	dial          func(ctx context.Context) (net.Conn, error)
	maxConcurrent int
	muxConfig     *smux.Config

	sm   sync.RWMutex
	sess map[*smux.Session]struct{}

	dm          sync.Mutex
	dialing     *dialCall
	dialWaiting int
}

func writeMuxHeader(dial func(ctx context.Context) (net.Conn, error), mode byte) func(ctx context.Context) (net.Conn, error) {
	return func(ctx context.Context) (net.Conn, error) {
		c, err := dial(ctx)
		if err != nil {
			return nil, err
		}
		if _, err := c.Write([]byte{mode}); err != nil {
			c.Close()
			return nil, err
		}
		return c, nil
	}
}

func NewMuxTransport(nextTransport Transport, maxConcurrent int, idleTimeout time.Duration) *MuxTransport {
	t := &MuxTransport{
		nextTransport: nextTransport,
	}

	switch {
	case maxConcurrent > 1:
		t.sMuxPool = &sMuxPool{
			dial:          writeMuxHeader(nextTransport.Dial, modeSMux),
			maxConcurrent: maxConcurrent,
			muxConfig:     getSMuxConf(idleTimeout),
		}
	case maxConcurrent == 1:
		t.keepalivePool = &keepalive.SessPool{
			DialContext: writeMuxHeader(nextTransport.Dial, modeKeepalive),
			Opt: &keepalive.Opt{
				IdleTimeout:           idleTimeout,
				IdleConnectionTimeout: idleTimeout,
			},
		}
	}
	return t
}

type dialCall struct {
	done chan struct{} // closed when dial is finished
	s    *smux.Session // only valid after done is closed
	err  error
}

func (m *MuxTransport) Dial(ctx context.Context) (net.Conn, error) {
	switch {
	case m.sMuxPool != nil:
		if stream := m.sMuxPool.tryGetStream(); stream != nil {
			return stream, nil
		}
		return m.sMuxPool.tryGetStreamFlash(ctx)
	case m.keepalivePool != nil:
		return m.keepalivePool.GetConn(ctx)
	default:
		return writeMuxHeader(m.nextTransport.Dial, modePlain)(ctx)
	}
}

func (p *sMuxPool) MarkDead(sess *smux.Session) {
	p.sm.Lock()
	defer p.sm.Unlock()
	delete(p.sess, sess)
	sess.Close()
}

func (p *sMuxPool) tryGetStream() (stream *smux.Stream) {
	for {
		p.sm.RLock()
		var sess *smux.Session
		for sess = range p.sess {
			if sess.NumStreams() < p.maxConcurrent {
				break
			}
		}
		p.sm.RUnlock()

		if sess == nil {
			return nil
		}

		s, err := sess.OpenStream()
		if err != nil {
			logMuxSessErr(sess, err)
			sess.Close()
			p.sm.Lock()
			delete(p.sess, sess)
			p.sm.Unlock()
			continue
		}
		return s
	}
}

func (p *sMuxPool) tryGetStreamFlash(ctx context.Context) (*smux.Stream, error) {
	var call *dialCall
	p.dm.Lock()
	if p.dialing == nil || (p.dialing != nil && p.dialWaiting >= p.maxConcurrent) {
		p.dialWaiting = 0
		p.dialing = p.dialSessLocked(ctx) // needs a new dial
	} else {
		p.dialWaiting++
	}
	call = p.dialing
	defer p.dm.Unlock()

	<-call.done
	sess := call.s
	err := call.err
	if err != nil {
		return nil, err
	}
	return sess.OpenStream()
}

func (p *sMuxPool) dialSessLocked(ctx context.Context) (call *dialCall) {
	call = &dialCall{
		done: make(chan struct{}),
	}
	go func() {
		c, err := p.dial(ctx)
		if err != nil {
			call.err = err
			close(call.done)
			return
		}

		sess, err := smux.Client(c, p.muxConfig)
		call.s = sess
		call.err = err
		close(call.done)

		p.sm.Lock()
		if p.sess == nil {
			p.sess = make(map[*smux.Session]struct{})
		}
		p.sess[sess] = struct{}{}
		p.sm.Unlock()

		p.dm.Lock()
		p.dialing = nil
		p.dm.Unlock()
	}()
	return call
}

type MuxTransportHandler struct {
	nextHandler  TransportHandler
	muxConfig    *smux.Config
	keepaliveOpt *keepalive.Opt
}

func NewMuxTransportHandler(nextHandler TransportHandler, idleTimeout time.Duration) *MuxTransportHandler {
	return &MuxTransportHandler{
		nextHandler: nextHandler,
		muxConfig:   getSMuxConf(idleTimeout),
		keepaliveOpt: &keepalive.Opt{
			AcceptNewConnectionFromPeer: true,
			IdleTimeout:                 idleTimeout,
			IdleConnectionTimeout:       idleTimeout,
		},
	}
}

func (h *MuxTransportHandler) Handle(conn net.Conn) error {
	header := make([]byte, 1)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("failed to read mux header: %w", err)
	}

	switch header[0] {
	case modePlain:
		return h.nextHandler.Handle(conn)
	case modeSMux:
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
	case modeKeepalive:
		sess := keepalive.NewSession(conn, h.keepaliveOpt)
		defer sess.Close()
		for {
			stream, err := sess.Accept()
			if err != nil {
				return nil // suppress this err
			}
			if err := h.nextHandler.Handle(stream); err != nil {
				logConnErr(stream, err)
			}
			stream.Close()
		}
	default:
		return fmt.Errorf("invalid mux header %d", header[0])
	}
}
