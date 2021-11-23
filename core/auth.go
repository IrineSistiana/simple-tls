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
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

type AuthTransport struct {
	nextTransport Transport
	auth          [md5.Size]byte
}

func (t *AuthTransport) Dial(ctx context.Context) (net.Conn, error) {
	conn, err := t.nextTransport.Dial(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(t.auth[:]); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to write auth: %w", err)
	}
	return conn, nil
}

func NewAuthTransport(nextTransport Transport, auth string) *AuthTransport {
	return &AuthTransport{nextTransport: nextTransport, auth: md5.Sum([]byte(auth))}
}

type AuthTransportHandler struct {
	nextHandler TransportHandler
	auth        [md5.Size]byte
}

func NewAuthTransportHandler(nextHandler TransportHandler, auth string) *AuthTransportHandler {
	return &AuthTransportHandler{nextHandler: nextHandler, auth: md5.Sum([]byte(auth))}
}

var errAuthFailed = errors.New("auth failed")

func (h *AuthTransportHandler) Handle(conn net.Conn) error {
	var auth [md5.Size]byte
	if _, err := io.ReadFull(conn, auth[:]); err != nil {
		return fmt.Errorf("failed to read auth header: %w", err)
	}

	if auth != h.auth {
		discardRead(conn, time.Second*15)
		return errAuthFailed
	}

	return h.nextHandler.Handle(conn)
}
