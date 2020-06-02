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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"
)

func doClient(l net.Listener, serverAddr, hostName string, caPool *x509.CertPool, wss bool, path string, sendRandomHeader bool, timeout time.Duration, vpnMode, tfo bool) error {
	dialer := net.Dialer{
		Timeout: time.Second * 5,
		Control: getControlFunc(&tcpConfig{vpnMode: vpnMode, tfo: tfo}),
	}
	tlsConfig := new(tls.Config)
	tlsConfig.ClientSessionCache = tls.NewLRUClientSessionCache(64)
	tlsConfig.ServerName = hostName
	tlsConfig.RootCAs = caPool

	var httpClient *http.Client
	var url string
	if wss {
		transport := &http.Transport{
			DialTLSContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				conn, err := dialer.DialContext(ctx, network, serverAddr)
				if err != nil {
					return nil, err
				}
				tlsConn := tls.Client(conn, tlsConfig)
				err = tls13HandshakeWithTimeout(tlsConn, time.Second*5)
				if err != nil {
					conn.Close()
					return nil, err
				}
				return tlsConn, nil
			},
			IdleConnTimeout:       time.Minute,
			ResponseHeaderTimeout: time.Second * 10,
			ForceAttemptHTTP2:     true,
		}

		err := http2.ConfigureTransport(transport) // enable http2
		if err != nil {
			return err
		}

		httpClient = &http.Client{
			Transport: transport,
		}

		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		url = "wss://" + hostName + path
	}

	for {
		localConn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("l.Accept(): %w", err)
		}

		go func() {
			defer localConn.Close()
			var serverConn net.Conn

			if wss {
				serverWSSConn, err := dialWebsocketConn(httpClient, url)
				if err != nil {
					log.Printf("ERROR: doClient: dialWebsocketConn: %v", err)
					return
				}
				defer serverWSSConn.Close()

				serverConn = serverWSSConn
			} else {
				serverRawConn, err := dialer.Dial("tcp", serverAddr)
				if err != nil {
					log.Printf("ERROR: doClient: dialer.Dial: %v", err)
					return
				}
				defer serverRawConn.Close()

				serverTLSConn := tls.Client(serverRawConn, tlsConfig)
				if err := tls13HandshakeWithTimeout(serverTLSConn, time.Second*5); err != nil {
					log.Printf("ERROR: doClient: tlsHandshakeTimeout: %v", err)
					return
				}

				serverConn = serverTLSConn
			}

			if sendRandomHeader {
				serverConn = &randomHeaderConn{Conn: serverConn}
			}

			if err := openTunnel(localConn, serverConn, timeout); err != nil {
				log.Printf("ERROR: doClient: openTunnel: %v", err)
			}
		}()
	}
}
