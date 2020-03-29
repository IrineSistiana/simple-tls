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
	"fmt"
	"log"
	"net"
	"time"
)

func doClient(l net.Listener, server string, tlsConfig *tls.Config, timeout time.Duration, vpnMode, tfo bool) error {
	dialer := net.Dialer{
		Control: getControlFunc(&tcpConfig{vpnMode: vpnMode, tfo: tfo}),
	}

	for {
		localConn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("l.Accept(): %w", err)
		}

		go func() {
			defer localConn.Close()

			serverRawConn, err := dialer.Dial("tcp", server)
			if err != nil {
				log.Printf("doClient: dialer.Dial: %v", err)
				return
			}
			defer serverRawConn.Close()

			serverTLSConn := tls.Client(serverRawConn, tlsConfig)
			if err := serverTLSConn.Handshake(); err != nil {
				log.Printf("doServer: serverTLSConn.Handshake: %v", err)
				return
			}

			if err := openTunnel(localConn, serverTLSConn, timeout); err != nil {
				log.Printf("doServer: openTunnel: %v", err)
			}
		}()
	}
}
