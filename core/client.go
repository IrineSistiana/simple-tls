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
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/IrineSistiana/ctunnel"
	"log"
	"net"
	"time"
)

type Client struct {
	Listener           net.Listener
	ServerAddr         string
	NoTLS              bool
	Auth               string
	ServerName         string
	CertPool           *x509.CertPool
	InsecureSkipVerify bool
	Timeout            time.Duration
	AndroidVPNMode     bool
	TFO                bool
	Mux                int

	dialer    net.Dialer
	auth      [16]byte
	tlsConfig *tls.Config
	muxPool   *muxPool // not nil if Mux > 0
}

func (c *Client) ActiveAndServe() error {
	c.dialer = net.Dialer{
		Timeout: time.Second * 5,
		Control: GetControlFunc(&TcpConfig{AndroidVPN: c.AndroidVPNMode, EnableTFO: c.TFO}),
	}

	if !c.NoTLS {
		c.tlsConfig = new(tls.Config)
		c.tlsConfig.NextProtos = []string{"http/1.1", "h2"}
		c.tlsConfig.ServerName = c.ServerName
		c.tlsConfig.RootCAs = c.CertPool
		c.tlsConfig.InsecureSkipVerify = c.InsecureSkipVerify
	}

	if len(c.Auth) > 0 {
		c.auth = md5.Sum([]byte(c.Auth))
	}

	if c.Mux > 0 {
		c.muxPool = newMuxPool(c.dialServerConn, c.Mux)
	}

	for {
		localConn, err := c.Listener.Accept()
		if err != nil {
			return fmt.Errorf("l.Accept(): %w", err)
		}
		reduceLoopbackSocketBuf(localConn)

		go func() {
			defer localConn.Close()

			var serverConn net.Conn
			if c.Mux > 0 {
				stream, _, err := c.muxPool.GetStream()
				if err != nil {
					log.Printf("ERROR: muxPool.GetStream: %v", err)
					return
				}
				serverConn = stream
			} else {
				conn, err := c.dialServerConn()
				if err != nil {
					log.Printf("ERROR: dialServerConn: %v", err)
					return
				}
				serverConn = conn
			}
			defer serverConn.Close()

			if err := ctunnel.OpenTunnel(localConn, serverConn, c.Timeout); err != nil {
				log.Printf("ERROR: ActiveAndServe: openTunnel: %v", err)
			}
		}()
	}
}

func (c *Client) dialServerConn() (net.Conn, error) {
	serverConn, err := c.dialer.Dial("tcp", c.ServerAddr)
	if err != nil {
		return nil, err
	}

	if !c.NoTLS {
		serverTLSConn := tls.Client(serverConn, c.tlsConfig)
		if err := tls13HandshakeWithTimeout(serverTLSConn, time.Second*5); err != nil {
			serverTLSConn.Close()
			return nil, err
		}
		serverConn = serverTLSConn
	}

	// write auth
	if len(c.Auth) > 0 {
		if _, err := serverConn.Write(c.auth[:]); err != nil {
			serverConn.Close()
			return nil, fmt.Errorf("failed to write auth: %w", err)
		}
	}

	// write mode
	mode := modePlain
	if c.Mux > 0 {
		mode = modeMux
	}
	if _, err := serverConn.Write([]byte{mode}); err != nil {
		serverConn.Close()
		return nil, fmt.Errorf("failed to write mode: %w", err)
	}

	return serverConn, nil
}
