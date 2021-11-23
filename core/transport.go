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
	"fmt"
	"github.com/IrineSistiana/ctunnel"
	"net"
	"time"
)

type Transport interface {
	Dial(ctx context.Context) (net.Conn, error)
}

type TransportHandler interface {
	Handle(conn net.Conn) error
}

type RawConnTransport struct {
	addr   string
	dialer *net.Dialer
}

func (t *RawConnTransport) Dial(ctx context.Context) (net.Conn, error) {
	return t.dialer.DialContext(ctx, "tcp", t.addr)
}

func NewRawConnTransport(addr string, dialer *net.Dialer) *RawConnTransport {
	return &RawConnTransport{addr: addr, dialer: dialer}
}

type TLSTransport struct {
	nextTransport Transport
	conf          *tls.Config
}

func (t *TLSTransport) Dial(ctx context.Context) (net.Conn, error) {
	conn, err := t.nextTransport.Dial(ctx)
	if err != nil {
		return nil, err
	}

	tlsConn := tls.Client(conn, t.conf)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		tlsConn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func NewTLSTransport(nextTransport Transport, conf *tls.Config) *TLSTransport {
	return &TLSTransport{nextTransport: nextTransport, conf: conf}
}

type BaseTransportHandler struct {
	dst         string
	idleTimeout time.Duration
}

func (h *BaseTransportHandler) Handle(conn net.Conn) error {
	dstConn, err := net.Dial("tcp", h.dst)
	if err != nil {
		return fmt.Errorf("cannot connect to the dst: %w", err)
	}
	reduceTCPLoopbackSocketBuf(dstConn)
	defer dstConn.Close()

	if err := ctunnel.OpenTunnel(dstConn, conn, h.idleTimeout); err != nil {
		return fmt.Errorf("tunnel closed: %w", err)
	}
	return nil
}

func NewBaseTransportHandler(dst string, idleTimeout time.Duration) *BaseTransportHandler {
	return &BaseTransportHandler{dst: dst, idleTimeout: idleTimeout}
}

func ListenRawConn(l net.Listener, nextHandler TransportHandler) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}

		go func() {
			defer conn.Close()
			err := nextHandler.Handle(conn)
			if err != nil {
				logConnErr(conn, err)
			}
		}()
	}
}
