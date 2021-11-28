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
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"time"
)

type Server struct {
	BindAddr string
	DstAddr  string

	Websocket     bool
	WebsocketPath string

	Cert, Key, ServerName string
	Auth                  string
	TFO                   bool
	IdleTimeout           time.Duration
	NoTLS                 bool

	testListener net.Listener
	testCert     *tls.Certificate
}

var errMissingCertOrKey = errors.New("one of cert or key argument is missing")

func (s *Server) ActiveAndServe() error {
	var l net.Listener
	if s.testListener != nil {
		l = s.testListener
	} else {
		var err error
		lc := net.ListenConfig{Control: GetControlFunc(&TcpConfig{EnableTFO: s.TFO})}
		l, err = lc.Listen(context.Background(), "tcp", s.BindAddr)
		if err != nil {
			return err
		}
	}

	var transportHandler TransportHandler
	transportHandler = NewBaseTransportHandler(s.DstAddr, s.IdleTimeout)
	transportHandler = NewMuxTransportHandler(transportHandler, s.IdleTimeout)
	if len(s.Auth) > 0 {
		transportHandler = NewAuthTransportHandler(transportHandler, s.Auth)
	}

	if !s.NoTLS {
		var certificate tls.Certificate
		if s.testCert != nil {
			certificate = *s.testCert
		} else {
			switch {
			case len(s.Cert) == 0 && len(s.Key) == 0: // no cert and key
				dnsName, _, keyPEM, certPEM, err := GenerateCertificate(s.ServerName)
				if err != nil {
					return fmt.Errorf("failed to generate temp cert: %w", err)
				}

				log.Printf("warnning: you are using a tmp certificate with dns name: %s", dnsName)
				cer, err := tls.X509KeyPair(certPEM, keyPEM)
				if err != nil {
					return fmt.Errorf("cannot load x509 key pair from memory: %w", err)
				}

				certificate = cer
			case len(s.Cert) != 0 && len(s.Key) != 0: // has a cert and a key
				cer, err := tls.LoadX509KeyPair(s.Cert, s.Key) //load cert
				if err != nil {
					return fmt.Errorf("cannot load x509 key pair from disk: %w", err)
				}
				certificate = cer
			default:
				return errMissingCertOrKey
			}
		}

		tlsConfig := &tls.Config{
			NextProtos:   []string{"h2", "http/1.1"},
			Certificates: []tls.Certificate{certificate},
			VerifyConnection: func(state tls.ConnectionState) error {
				if state.Version != tls.VersionTLS13 {
					return fmt.Errorf("unsafe tls version %d", state.Version)
				}
				return nil
			},
		}

		l = tls.NewListener(l, tlsConfig)
	}

	if s.Websocket {
		return ListenWebsocket(l, s.WebsocketPath, transportHandler)
	}
	return ListenRawConn(l, transportHandler)
}
