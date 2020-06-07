// +build linux android

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

package main

import (
	"log"

	"golang.org/x/sys/unix"
)

//TCP_MAXSEG TCP_NODELAY SO_SND/RCVBUF etc..
func (c *tcpConfig) setSockOpt(uintFd uintptr) {
	if c == nil {
		return
	}
	fd := int(uintFd)

	if c.tfo {
		err := unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_FASTOPEN_CONNECT, 1)
		if err != nil {
			log.Printf("setsockopt: TCP_FASTOPEN_CONNECT, %v", err)
		}
	}
	return
}
