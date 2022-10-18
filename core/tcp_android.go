//go:build android

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

import "C"
import (
	"golang.org/x/sys/unix"
	"log"
	"net"
	"syscall"
	"time"
)

func GetControlFunc(conf *TcpConfig) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		if conf.AndroidVPN {
			var scmErr error
			if err := c.Control(func(fd uintptr) {
				scmErr = sendFdToVPN(fd)
			}); err != nil {
				return err
			}
			if scmErr != nil {
				log.Printf("failed to protect self conn from vpn service, %v", scmErr)
				return scmErr
			}
		}
		if conf != nil {
			return c.Control(conf.setSockOpt)
		} else {
			return nil
		}
	}
}

func sendFdToVPN(fd uintptr) error {
	const vpnPath = "protect_path"
	uc, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: vpnPath})
	if err != nil {
		return err
	}
	defer uc.Close()
	uc.SetDeadline(time.Now().Add(time.Second))
	_, _, err = uc.WriteMsgUnix(nil, unix.UnixRights(int(fd)), nil)
	if err != nil {
		return err
	}
	if _, err := uc.Read([]byte{0}); err != nil {
		return err
	}
	return nil
}
