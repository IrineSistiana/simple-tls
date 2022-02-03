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
	"errors"
	"net"
	"sync"
)

type SessPool struct {
	DialContext func(ctx context.Context) (net.Conn, error)
	Opt         *Opt

	pm   sync.Mutex
	sess map[*Session]struct{}
}

type autoReleaseConn struct {
	*Stream

	sess *Session
	pool *SessPool

	closeOnce sync.Once
}

func warpStream(stream *Stream, sess *Session, pool *SessPool) *autoReleaseConn {
	return &autoReleaseConn{Stream: stream, sess: sess, pool: pool}
}

func (c *autoReleaseConn) Close() error {
	c.closeOnce.Do(func() {
		c.Stream.Close()
		c.pool.releaseSession(c.sess)
	})
	return nil
}

func (p *SessPool) GetConn(ctx context.Context) (net.Conn, error) {
	stream, session, ok := p.tryGetFromPool()
	if ok {
		return warpStream(stream, session, p), nil
	}

	if p.Opt != nil && p.Opt.AcceptNewConnectionFromPeer {
		return nil, errors.New("pool cannot accept new connections from peer")
	}

	conn, err := p.DialContext(ctx)
	if err != nil {
		return nil, err
	}

	session = NewSession(conn, p.Opt)
	stream, err = session.Open()
	if err != nil {
		session.Close()
		return nil, err
	}
	return warpStream(stream, session, p), nil
}

func (p *SessPool) releaseSession(sess *Session) {
	p.pm.Lock()
	defer p.pm.Unlock()

	if p.sess == nil {
		p.sess = make(map[*Session]struct{})
	}

	if !sess.Idle() {
		panic("release a occupied session")
	}
	p.sess[sess] = struct{}{}
}

func (p *SessPool) tryGetFromPool() (*Stream, *Session, bool) {
	p.pm.Lock()
	defer p.pm.Unlock()

	for session := range p.sess {
		stream, err := session.Open()
		if err != nil {
			session.Close()
			delete(p.sess, session)
			continue
		}
		delete(p.sess, session)
		return stream, session, true
	}
	return nil, nil, false
}
