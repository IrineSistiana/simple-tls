//     Copyright (C) 2020, IrineSistiana
//
//     This file is part of simple-tls.
//
//     mosdns is free software: you can redistribute it and/or modify
//     it under the terms of the GNU General Public License as published by
//     the Free Software Foundation, either version 3 of the License, or
//     (at your option) any later version.
//
//     mosdns is distributed in the hope that it will be useful,
//     but WITHOUT ANY WARRANTY; without even the implied warranty of
//     MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//     GNU General Public License for more details.
//
//     You should have received a copy of the GNU General Public License
//     along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"crypto/tls"
	"errors"
	"time"
)

var (
	errNotTLS13 = errors.New("not a tls 1.3 connection")
)

func tls13HandshakeWithTimeout(c *tls.Conn, timeout time.Duration) error {
	c.SetDeadline(time.Now().Add(timeout))
	if err := c.Handshake(); err != nil {
		return err
	}
	c.SetDeadline(time.Time{})
	if c.ConnectionState().Version != tls.VersionTLS13 {
		return errNotTLS13
	}
	return nil
}
