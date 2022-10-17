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
	"github.com/IrineSistiana/simple-tls/core/ctunnel"
	"net"
	"time"
)

type Transport interface {
	Dial(ctx context.Context) (net.Conn, error)
}

type TransportHandler interface {
	Handle(conn net.Conn) error
}

type DstTransportHandler struct {
	dst             string
	idleTimeout     time.Duration
	outboundBufSize int
}

func (h *DstTransportHandler) Handle(conn net.Conn) error {
	dstConn, err := net.Dial("tcp", h.dst)
	if err != nil {
		return fmt.Errorf("cannot connect to the dst: %w", err)
	}
	defer dstConn.Close()
	applyTCPSocketBuf(dstConn, h.outboundBufSize)
	if err := ctunnel.OpenTunnel(dstConn, conn, ctunnel.TunnelOpts{IdleTimout: h.idleTimeout}); err != nil {
		return fmt.Errorf("tunnel closed: %w", err)
	}
	return nil
}

func NewDstTransportHandler(dst string, idleTimeout time.Duration, outboundBufSize int) *DstTransportHandler {
	return &DstTransportHandler{dst: dst, idleTimeout: idleTimeout, outboundBufSize: outboundBufSize}
}

func ListenRawConn(l net.Listener, nextHandler TransportHandler) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go func() {
			defer conn.Close()

			if tlsConn, ok := conn.(*tls.Conn); ok {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
				err := tlsConn.HandshakeContext(ctx)
				cancel()
				if err != nil {
					logConnErr(conn, err)
					return
				}
			}
			err := nextHandler.Handle(conn)
			if err != nil {
				logConnErr(conn, err)
			}
		}()
	}
}
