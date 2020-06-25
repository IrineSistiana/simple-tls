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
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"time"
)

func DoServer(l net.Listener, certificates []tls.Certificate, dst string, sendPaddingData bool, timeout time.Duration) error {

	tlsConfig := new(tls.Config)
	tlsConfig.MinVersion = tls.VersionTLS13
	tlsConfig.NextProtos = []string{"h2"}
	tlsConfig.Certificates = certificates
	tlsConfig.MinVersion = tls.VersionTLS13

	for {
		clientRawConn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("l.Accept(): %w", err)
		}

		go func() {
			clientTLSConn := tls.Server(clientRawConn, tlsConfig)
			defer clientTLSConn.Close()

			// check client conn before dialing dst
			if err := tls13HandshakeWithTimeout(clientTLSConn, time.Second*5); err != nil {
				log.Printf("ERROR: DoServer: %s, tls13HandshakeWithTimeout: %v", clientRawConn.RemoteAddr(), err)
				return
			}

			if err := handleClientConn(clientTLSConn, sendPaddingData, dst, timeout); err != nil {
				log.Printf("ERROR: DoServer: %s, handleClientConn: %v", clientRawConn.RemoteAddr(), err)
				return
			}
		}()
	}
}

func handleClientConn(cc net.Conn, sendPaddingData bool, dst string, timeout time.Duration) (err error) {
	dstConn, err := net.Dial("tcp", dst)
	if err != nil {
		return fmt.Errorf("net.Dial: %v", err)
	}
	defer dstConn.Close()

	if sendPaddingData {
		cc = newPaddingConn(cc, false, true)
	}

	if err := openTunnel(dstConn, cc, timeout); err != nil {
		return fmt.Errorf("openTunnel: %v", err)
	}
	return nil
}
