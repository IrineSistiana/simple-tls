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
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"time"
)

func DoClient(l net.Listener, serverAddr, hostName string, caPool *x509.CertPool, insecureSkipVerify, sendPaddingData bool, timeout time.Duration, vpnMode, tfo bool) error {
	dialer := net.Dialer{
		Timeout: time.Second * 5,
		Control: GetControlFunc(&TcpConfig{AndroidVPN: vpnMode, EnableTFO: tfo}),
	}
	tlsConfig := new(tls.Config)
	tlsConfig.ClientSessionCache = tls.NewLRUClientSessionCache(64)
	tlsConfig.ServerName = hostName
	tlsConfig.RootCAs = caPool
	tlsConfig.InsecureSkipVerify = insecureSkipVerify

	for {
		localConn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("l.Accept(): %w", err)
		}

		go func() {
			defer localConn.Close()

			serverRawConn, err := dialer.Dial("tcp", serverAddr)
			if err != nil {
				log.Printf("ERROR: DoClient: dialer.Dial: %v", err)
				return
			}
			defer serverRawConn.Close()

			serverTLSConn := tls.Client(serverRawConn, tlsConfig)
			if err := tls13HandshakeWithTimeout(serverTLSConn, time.Second*5); err != nil {
				log.Printf("ERROR: DoClient: tlsHandshakeTimeout: %v", err)
				return
			}

			var serverConn net.Conn
			if sendPaddingData {
				serverConn = newPaddingConn(serverTLSConn, true, false)
			} else {
				serverConn = serverTLSConn
			}

			if err := openTunnel(localConn, serverConn, timeout); err != nil {
				log.Printf("ERROR: DoClient: openTunnel: %v", err)
			}
		}()
	}
}
