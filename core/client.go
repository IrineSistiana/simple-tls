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
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/IrineSistiana/ctunnel"
	"io/ioutil"
	"net"
	"strings"
	"time"
)

type Client struct {
	BindAddr      string
	DstAddr       string
	Websocket     bool
	WebsocketPath string
	Mux           int
	Auth          string

	ServerName         string
	CA                 string
	CertHash           string
	InsecureSkipVerify bool

	IdleTimeout time.Duration
	SocketOpts  *TcpConfig

	testListener net.Listener
}

var errEmptyCAFile = errors.New("no valid certificate was found in the ca file")

func (c *Client) ActiveAndServe() error {

	var l net.Listener
	if c.testListener != nil {
		l = c.testListener
	} else {
		var err error
		lc := net.ListenConfig{}
		l, err = lc.Listen(context.Background(), "tcp", c.BindAddr)
		if err != nil {
			return err
		}
	}

	if len(c.ServerName) == 0 {
		c.ServerName = strings.SplitN(c.DstAddr, ":", 2)[0]
	}

	var rootCAs *x509.CertPool
	if len(c.CA) != 0 {
		rootCAs = x509.NewCertPool()
		certPEMBlock, err := ioutil.ReadFile(c.CA)
		if err != nil {
			return fmt.Errorf("cannot read ca file: %w", err)
		}
		if ok := rootCAs.AppendCertsFromPEM(certPEMBlock); !ok {
			return errEmptyCAFile
		}
	}

	dialer := &net.Dialer{
		Timeout: time.Second * 5,
		Control: GetControlFunc(c.SocketOpts),
	}

	var chb []byte
	if len(c.CertHash) != 0 {
		b, err := hex.DecodeString(c.CertHash)
		if err != nil {
			return fmt.Errorf("invalid cert hash: %w", err)
		}
		chb = b
	}

	tlsConfig := &tls.Config{
		ServerName:         c.ServerName,
		RootCAs:            rootCAs,
		InsecureSkipVerify: c.InsecureSkipVerify,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(chb) != 0 {
				cert := state.PeerCertificates[0]
				h := sha256.Sum256(cert.RawTBSCertificate)
				if bytes.Equal(h[:len(chb)], chb) {
					return nil
				}
				return fmt.Errorf("cert hash mismatch, recieved cert hash is [%s]", hex.EncodeToString(h[:]))
			}

			if state.Version != tls.VersionTLS13 {
				return fmt.Errorf("unsafe tls version %d", state.Version)
			}
			return nil
		},
	}

	var transport Transport
	if c.Websocket {
		tlsConfig.NextProtos = []string{"http/1.1"}
		transport = NewWebsocketTransport(c.DstAddr, c.ServerName, c.WebsocketPath, tlsConfig, dialer)
	} else {
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
		transport = NewRawConnTransport(c.DstAddr, dialer)
		transport = NewTLSTransport(transport, tlsConfig)
	}

	if len(c.Auth) > 0 {
		transport = NewAuthTransport(transport, c.Auth)
	}

	transport = NewMuxTransport(transport, c.Mux, c.IdleTimeout)

	for {
		clientConn, err := l.Accept()
		if err != nil {
			return err
		}

		go func() {
			defer clientConn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			serverConn, err := transport.Dial(ctx)
			if err != nil {
				errLogger.Printf("failed to dial server connection: %v", err)
				return
			}
			defer serverConn.Close()

			err = ctunnel.OpenTunnel(clientConn, serverConn, c.IdleTimeout)
			if err != nil {
				logConnErr(clientConn, fmt.Errorf("tunnel closed: %w", err))
			}
		}()
	}

}
