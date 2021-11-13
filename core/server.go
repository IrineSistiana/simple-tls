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
	NoTLS    bool
	Auth     string

	Certificates []tls.Certificate
	Timeout      time.Duration

	auth      [16]byte
	tlsConfig *tls.Config
}

func (s *Server) ActiveAndServe() error {
	if !s.NoTLS {
		s.tlsConfig = new(tls.Config)
		s.tlsConfig.NextProtos = []string{"h2"}
		s.tlsConfig.Certificates = s.Certificates
	}

	if len(s.Auth) > 0 {
		s.auth = md5.Sum([]byte(s.Auth))
	}

	for {
		clientConn, err := s.Listener.Accept()
		if err != nil {
			return fmt.Errorf("l.Accept(): %w", err)
		}

		go func() {
			defer clientConn.Close()

			if !s.NoTLS {
				clientTLSConn := tls.Server(clientConn, s.tlsConfig)
				// handshake
				if err := tls13HandshakeWithTimeout(clientTLSConn, time.Second*5); err != nil {
					log.Printf("ERROR: %s, tls13HandshakeWithTimeout: %v", clientConn.RemoteAddr(), err)
					return
				}
				clientConn = clientTLSConn
			}

			// check auth
			if len(s.Auth) > 0 {
				auth := make([]byte, 16)
				if _, err := io.ReadFull(clientConn, auth); err != nil {
					log.Printf("ERROR: %s, read client auth header: %v", clientConn.RemoteAddr(), err)
					return
				}

				if !bytes.Equal(s.auth[:], auth) {
					log.Printf("ERROR: %s, auth failed", clientConn.RemoteAddr())
					discardRead(clientConn, time.Second*15)
					return
				}
			}

			// mode
			header := make([]byte, 1)
			if _, err := io.ReadFull(clientConn, header); err != nil {
				log.Printf("ERROR: %s, read client mode header: %v", clientConn.RemoteAddr(), err)
				return
			}

			switch header[0] {
			case modePlain:
				if err := s.handleClientConn(clientConn); err != nil {
					log.Printf("ERROR: %s, handleClientConn: %v", clientConn.RemoteAddr(), err)
					return
				}
			case modeMux:
				err := s.handleClientMux(clientConn)
				if err != nil {
					log.Printf("ERROR: %s, handleClientMux: %v", clientConn.RemoteAddr(), err)
					return
				}
			default:
				log.Printf("ERROR: %s, invalid header %d", clientConn.RemoteAddr(), header[0])
				return
			}
		}()
	}
}

func discardRead(c net.Conn, t time.Duration) {
	c.SetDeadline(time.Now().Add(t))
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
	reduceTCPLoopbackSocketBuf(dstConn)
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
