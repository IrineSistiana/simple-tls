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
	"sync/atomic"
	"time"
)

type conn struct {
	net.Conn
	trackReadTime atomic.Value
	idleTimeout   time.Duration
}

func (c *conn) getLastReadTime() time.Time {
	t, _ := c.trackReadTime.Load().(time.Time)
	return t
}

func (c *conn) Read(p []byte) (int, error) {
	if c.idleTimeout > 0 {
		c.Conn.SetDeadline(time.Now().Add(c.idleTimeout))
	}

	n, err := c.Conn.Read(p)

	if n > 0 {
		c.trackReadTime.Store(time.Now())
	}
	return n, err
}

func (c *conn) Write(p []byte) (int, error) {
	if c.idleTimeout > 0 {
		c.Conn.SetDeadline(time.Now().Add(c.idleTimeout))
	}
	return c.Conn.Write(p)
}
