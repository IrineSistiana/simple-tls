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
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"fmt"
	"github.com/IrineSistiana/ctunnel"
	"github.com/xtaci/smux"
	"io"
	"log"
	"net"
	"time"
)

type Server struct {
	Listener net.Listener
	Dst      string
	Auth     string

	Certificates []tls.Certificate
	Timeout      time.Duration

	auth [16]byte
}

func (s *Server) ActiveAndServe() error {
	tlsConfig := new(tls.Config)
	tlsConfig.NextProtos = []string{"h2"}
	tlsConfig.Certificates = s.Certificates

	if len(s.Auth) > 0 {
		s.auth = md5.Sum([]byte(s.Auth))
	}

	for {
		clientRawConn, err := s.Listener.Accept()
		if err != nil {
			return fmt.Errorf("l.Accept(): %w", err)
		}

		go func() {
			clientTLSConn := tls.Server(clientRawConn, tlsConfig)
			defer clientTLSConn.Close()

			// handshake
			if err := tls13HandshakeWithTimeout(clientTLSConn, time.Second*5); err != nil {
				log.Printf("ERROR: %s, tls13HandshakeWithTimeout: %v", clientRawConn.RemoteAddr(), err)
				return
			}

			// check auth
			if len(s.Auth) > 0 {
				auth := make([]byte, 16)
				if _, err := io.ReadFull(clientTLSConn, auth); err != nil {
					log.Printf("ERROR: %s, read client auth header: %v", clientRawConn.RemoteAddr(), err)
					return
				}

				if !bytes.Equal(s.auth[:], auth) {
					log.Printf("ERROR: %s, auth failed", clientRawConn.RemoteAddr())
					discard(clientTLSConn)
					return
				}
			}

			// mode
			header := make([]byte, 1)
			if _, err := io.ReadFull(clientTLSConn, header); err != nil {
				log.Printf("ERROR: %s, read client mode header: %v", clientRawConn.RemoteAddr(), err)
				return
			}

			switch header[0] {
			case modePlain:
				if err := s.handleClientConn(clientTLSConn); err != nil {
					log.Printf("ERROR: %s, handleClientConn: %v", clientRawConn.RemoteAddr(), err)
					return
				}
			case modeMux:
				err := s.handleClientMux(clientTLSConn)
				if err != nil {
					log.Printf("ERROR: %s, handleClientMux: %v", clientRawConn.RemoteAddr(), err)
					return
				}
			default:
				log.Printf("ERROR: %s, invalid header %d", clientRawConn.RemoteAddr(), header[0])
				return
			}
		}()
	}
}

func discard(c net.Conn) {
	c.SetDeadline(time.Now().Add(time.Second * 15))
	buf := make([]byte, 512)
	for {
		_, err := c.Read(buf)
		if err != nil {
			return
		}
	}
}

func (s *Server) handleClientConn(cc net.Conn) (err error) {
	dstConn, err := net.Dial("tcp", s.Dst)
	if err != nil {
		return fmt.Errorf("net.Dial: %v", err)
	}
	defer dstConn.Close()

	if err := ctunnel.OpenTunnel(dstConn, cc, s.Timeout); err != nil {
		return fmt.Errorf("openTunnel: %v", err)
	}
	return nil
}

func (s *Server) handleClientMux(cc net.Conn) (err error) {
	sess, err := smux.Server(cc, muxConfig)
	if err != nil {
		return err
	}
	defer sess.Close()

	for {
		stream, err := sess.AcceptStream()
		if err != nil {
			return nil // suppress smux err
		}
		go func() {
			defer stream.Close()
			if err := s.handleClientConn(stream); err != nil {
				log.Printf("ERROR: handleClientMux: %s, handleClientConn: %v", stream.RemoteAddr(), err)
				return
			}
		}()
	}
}
