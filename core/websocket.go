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
	"golang.org/x/net/http2"
	"net"
	"net/http"
	"net/url"
	"nhooyr.io/websocket"
	"time"
)

type WebsocketTransport struct {
	u  string
	op *websocket.DialOptions
}

func NewWebsocketTransport(serverAddr, serverName, urlPath string, tlsConfig *tls.Config, dialer *net.Dialer) *WebsocketTransport {
	u := url.URL{
		Scheme: "https",
		Host:   serverName,
		Path:   urlPath,
	}
	t := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, serverAddr)
		},
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   time.Second * 5,
		DisableCompression:    true,
		ResponseHeaderTimeout: time.Second * 5,
		ExpectContinueTimeout: 0,
		WriteBufferSize:       32 * 1024,
		ReadBufferSize:        32 * 1024,
		ForceAttemptHTTP2:     true,
	}

	t2, _ := http2.ConfigureTransports(t)
	t2.ReadIdleTimeout = time.Second * 30
	t2.PingTimeout = time.Second * 10

	return &WebsocketTransport{
		u: u.String(),
		op: &websocket.DialOptions{
			HTTPClient: &http.Client{
				Transport: t,
				Timeout:   time.Second * 10,
			},
			CompressionMode: websocket.CompressionDisabled,
		},
	}
}

func (p *WebsocketTransport) Dial(ctx context.Context) (net.Conn, error) {
	wsConn, _, err := websocket.Dial(ctx, p.u, p.op)
	if err != nil {
		return nil, err
	}
	return websocket.NetConn(context.Background(), wsConn, websocket.MessageBinary), nil
}

type wsHttpHandler struct {
	nextHandler TransportHandler
	path        string
}

var errInvalidPath = errors.New("invalid request path")

func (h *wsHttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(h.path) != 0 && r.URL.Path != h.path {
		w.WriteHeader(http.StatusNotFound)
		logRequestErr(r, errInvalidPath)
		return
	}

	wsConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		logRequestErr(r, fmt.Errorf("cannot accept websocket connection: %w", err))
		return
	}

	clientConn := websocket.NetConn(context.Background(), wsConn, websocket.MessageBinary)
	defer clientConn.Close()

	if err := h.nextHandler.Handle(clientConn); err != nil {
		logRequestErr(r, err)
		return
	}
	return
}

func ListenWebsocket(l net.Listener, path string, nextHandler TransportHandler) error {
	httpServer := &http.Server{
		Handler: &wsHttpHandler{
			nextHandler: nextHandler,
			path:        path,
		},
		ReadTimeout:       time.Second * 10,
		ReadHeaderTimeout: time.Second * 10,
		WriteTimeout:      time.Second * 10,
	}

	http2.ConfigureServer(httpServer, &http2.Server{
		IdleTimeout: time.Second * 45,
	})

	return httpServer.Serve(l)
}
