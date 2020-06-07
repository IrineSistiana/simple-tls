// +build android

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
	"syscall"

	"golang.org/x/sys/unix"
)

func getControlFunc(conf *tcpConfig) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		if conf.vpnMode {
			if err := c.Control(sendFdToBypass); err != nil {
				return err
			}
		}
		if conf != nil {
			return c.Control(conf.setSockOpt)
		} else {
			return nil
		}
	}
}

var protectPath = "protect_path"
var unixAddr = &unix.SockaddrUnix{Name: protectPath}
var unixTimeout = &unix.Timeval{Sec: 3, Usec: 0}

func sendFdToBypass(fd uintptr) {

	socket, err := unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		log.Printf("sendFdToBypass: Socket: %v", err)
		return
	}
	defer unix.Close(socket)

	unix.SetsockoptTimeval(socket, unix.SOL_SOCKET, unix.SO_RCVTIMEO, unixTimeout)
	unix.SetsockoptTimeval(socket, unix.SOL_SOCKET, unix.SO_SNDTIMEO, unixTimeout)

	err = unix.Connect(socket, unixAddr)
	if err != nil {
		log.Printf("sendFdToBypass: Connect: %v", err)
		return
	}

	//send fd
	if err := unix.Sendmsg(socket, nil, unix.UnixRights(int(fd)), nil, 0); err != nil {
		log.Printf("sendFdToBypass: Sendmsg: %v", err)
		return
	}

	//Read test ???
	unix.Read(socket, make([]byte, 1))
	return
}
