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
	"net"
	"sync"
	"time"
)

type connPool struct {
	sync.Mutex
	maxSize         int
	ttl             time.Duration
	cleanerInterval time.Duration

	pool      []*connInPool
	lastClean time.Time
}

type connInPool struct {
	c        net.Conn
	lastRead time.Time
}

func newConnPool(size int, ttl, gcInterval time.Duration) *connPool {
	return &connPool{
		maxSize:         size,
		ttl:             ttl,
		cleanerInterval: gcInterval,
		pool:            make([]*connInPool, 0),
	}

}

// runCleaner must run under lock
func (p *connPool) runCleaner(force bool) {
	if p.disabled() || len(p.pool) == 0 {
		return
	}

	//scheduled or forced
	if force || time.Since(p.lastClean) > p.cleanerInterval {
		p.lastClean = time.Now()
		res := p.pool[:0]
		for i := range p.pool {
			// remove expired conns
			if time.Since(p.pool[i].lastRead) < p.ttl {
				res = append(res, p.pool[i])
			} else { // expired, release the resources
				p.pool[i].c.Close()
				p.pool[i] = nil
			}
		}
		p.pool = res
	}

	//when the pool is full
	if len(p.pool) >= p.maxSize {
		res := p.pool[:0]
		mid := len(p.pool) >> 1
		for i := range p.pool {
			// remove half of the connections first
			if i < mid {
				p.pool[i].c.Close()
				p.pool[i] = nil
				continue
			}

			// then remove expired connections
			if time.Since(p.pool[i].lastRead) < p.ttl {
				res = append(res, p.pool[i])
			} else {
				p.pool[i].c.Close()
				p.pool[i] = nil
			}
		}
		p.pool = res
	}
}

func (p *connPool) put(c net.Conn) {
	if c == nil {
		return
	}

	if p.disabled() {
		c.Close()
		return
	}

	p.Lock()
	defer p.Unlock()

	p.runCleaner(false)

	if len(p.pool) >= p.maxSize {
		c.Close() // pool is full, drop it
	} else {
		p.pool = append(p.pool, &connInPool{c: c, lastRead: time.Now()})
	}
}

func (p *connPool) get() net.Conn {
	if p.disabled() {
		return nil
	}

	p.Lock()
	defer p.Unlock()

	p.runCleaner(false)

	if len(p.pool) > 0 {
		e := p.pool[len(p.pool)-1]
		p.pool[len(p.pool)-1] = nil
		p.pool = p.pool[:len(p.pool)-1]

		if time.Since(e.lastRead) > p.ttl {
			e.c.Close() // expired
			// the last elem is expired, means all elems are expired
			// remove them asap
			p.runCleaner(true)
			return nil
		}
		return e.c
	}
	return nil
}

func (p *connPool) disabled() bool {
	return p == nil || p.maxSize <= 0 || p.ttl <= 0
}
