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

import "net"

type TcpConfig struct {
	AndroidVPN bool
	EnableTFO  bool
}

func reduceLoopbackSocketBuf(c net.Conn) {
	tcpConn, ok := c.(*net.TCPConn)
	if ok && isLoopbackConn(tcpConn) {
		tcpConn.SetReadBuffer(32 * 1024)
		tcpConn.SetWriteBuffer(32 * 1024)
	}
}

func isLoopbackConn(c *net.TCPConn) bool {
	return c.LocalAddr().(*net.TCPAddr).IP.IsLoopback() || c.RemoteAddr().(*net.TCPAddr).IP.IsLoopback()
}
