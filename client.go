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
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func doClient(l net.Listener, server string, tlsConfig *tls.Config, wss bool, path string, timeout time.Duration, vpnMode, tfo bool) error {
	dialer := net.Dialer{
		Timeout: time.Second * 5,
		Control: getControlFunc(&tcpConfig{vpnMode: vpnMode, tfo: tfo}),
	}

	wsDialer := &websocket.Dialer{
		TLSClientConfig: tlsConfig,
		NetDial: func(network, _ string) (net.Conn, error) {
			// overwrite url host addr
			return dialer.Dial(network, server)
		},
		WriteBufferPool:  &sync.Pool{},
		HandshakeTimeout: time.Second * 8,
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	url := "wss://" + tlsConfig.ServerName + path

	for {
		localConn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("l.Accept(): %w", err)
		}

		go func() {
			defer localConn.Close()
			var serverConn net.Conn

			if wss {
				serverWSSConn, err := dialWebsocketConn(wsDialer, url)
				if err != nil {
					log.Printf("ERROR: doClient: dialWebsocketConn: %v", err)
					return
				}
				defer serverWSSConn.Close()

				serverConn = serverWSSConn
			} else {
				serverRawConn, err := dialer.Dial("tcp", server)
				if err != nil {
					log.Printf("ERROR: doClient: dialer.Dial: %v", err)
					return
				}
				defer serverRawConn.Close()

				serverTLSConn := tls.Client(serverRawConn, tlsConfig)

				if err := tlsHandshakeTimeout(serverTLSConn, time.Second*5); err != nil {
					log.Printf("ERROR: doClient: tlsHandshakeTimeout: %v", err)
					return
				}

				serverConn = serverTLSConn
			}

			if err := openTunnel(localConn, serverConn, timeout); err != nil {
				log.Printf("ERROR: doClient: openTunnel: %v", err)
			}
		}()
	}
}
