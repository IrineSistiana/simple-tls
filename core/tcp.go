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
	"net"
)

type TcpConfig struct {
	AndroidVPN bool
}

// applyTCPSocketBuf set tcp socket io buf if c is a *net.TCPConn.
// If c is a loopback conn, a 64k buf size will be applied. Otherwise,
// the buf size is set to userConfig.
// If userConfig <=0 applyTCPSocketBuf is a noop.
func applyTCPSocketBuf(c net.Conn, userConfig int) {
	tcpConn, ok := c.(*net.TCPConn)
	if ok {
		if isLocalConn(tcpConn) {
			tcpConn.SetReadBuffer(64 * 1024)
			tcpConn.SetWriteBuffer(64 * 1024)
		} else if userConfig > 0 {
			tcpConn.SetReadBuffer(userConfig)
			tcpConn.SetWriteBuffer(userConfig)
		}

	}
}

func isLocalConn(c *net.TCPConn) bool {
	la, ok := c.LocalAddr().(*net.TCPAddr)
	if !ok {
		return false
	}
	ra, ok := c.RemoteAddr().(*net.TCPAddr)
	if !ok {
		return false
	}
	return la.IP.IsLoopback() || ra.IP.IsLoopback()
}
