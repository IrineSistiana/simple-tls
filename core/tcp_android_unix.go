//go:build linux || android
// +build linux android

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
	"log"

	"golang.org/x/sys/unix"
)

func (c *TcpConfig) setSockOpt(uintFd uintptr) {
	if c == nil {
		return
	}
	fd := int(uintFd)

	if c.EnableTFO {
		err := unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_FASTOPEN_CONNECT, 1)
		if err != nil {
			log.Printf("setsockopt: TCP_FASTOPEN_CONNECT, %v", err)
		}
	}

	if c.TTL > 0 && c.TTL < 255 {
		err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_TTL, c.TTL)
		if err != nil {
			log.Printf("setsockopt: IP_TTL, %v", err)
		}
	}
	return
}
