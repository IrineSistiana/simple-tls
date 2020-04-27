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
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

// webSocketConnWrapper is a wrapper for net.Conn over WebSocket connection.
type webSocketConnWrapper struct {
	ws     *websocket.Conn
	reader io.Reader
}

func wrapWebSocketConn(c *websocket.Conn) net.Conn {
	return &webSocketConnWrapper{ws: c}
}

func dialWebsocketConn(d *websocket.Dialer, url string) (net.Conn, error) {
	c, _, err := d.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	uc := c.UnderlyingConn()
	if tlsConn, ok := uc.(*tls.Conn); ok {
		if err := tlsHandshakeTimeout(tlsConn, time.Second*5); err != nil {
			c.Close()
			return nil, fmt.Errorf("tlsHandshakeTimeout: %v", err)
		}
	}
	return wrapWebSocketConn(c), err
}

// Read implements io.Reader.
func (c *webSocketConnWrapper) Read(b []byte) (int, error) {
	n, err := c.read(b)
	if websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
		err = io.EOF
	}
	return n, err
}

func (c *webSocketConnWrapper) read(b []byte) (int, error) {
	var err error
	for {
		//previous reader reach the EOF, get next reader
		if c.reader == nil {
			//always BinaryMessage
			_, c.reader, err = c.ws.NextReader()
			if err != nil {
				return 0, err
			}
		}

		n, err := c.reader.Read(b)
		if n == 0 && err == io.EOF {
			c.reader = nil
			continue //nothing left in this reader
		}
		return n, err
	}
}

// Write implements io.Writer.
func (c *webSocketConnWrapper) Write(b []byte) (int, error) {
	if err := c.ws.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *webSocketConnWrapper) Close() error {
	return c.ws.Close()
}

func (c *webSocketConnWrapper) LocalAddr() net.Addr {
	return c.ws.LocalAddr()
}

func (c *webSocketConnWrapper) RemoteAddr() net.Addr {
	return c.ws.RemoteAddr()
}

func (c *webSocketConnWrapper) SetDeadline(t time.Time) error {
	return c.ws.UnderlyingConn().SetDeadline(t)
}

func (c *webSocketConnWrapper) SetReadDeadline(t time.Time) error {
	return c.ws.SetReadDeadline(t)
}

func (c *webSocketConnWrapper) SetWriteDeadline(t time.Time) error {
	return c.ws.SetWriteDeadline(t)
}
