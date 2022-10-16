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
	"github.com/IrineSistiana/simple-tls/core/ctunnel"
	"github.com/IrineSistiana/simple-tls/core/grpc_tunnel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"net"
	"os"
	"strings"
	"time"
)

type Client struct {
	BindAddr string
	DstAddr  string
	GRPC     bool
	GRPCAuth string

	ServerName         string
	CA                 string
	CertHash           string
	InsecureSkipVerify bool

	IdleTimeout time.Duration
	SocketOpts  *TcpConfig
	OutboundBuf int
	InboundBuf  int

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
	l = wrapListener(l, c.InboundBuf)

	if len(c.ServerName) == 0 {
		c.ServerName = strings.SplitN(c.DstAddr, ":", 2)[0]
	}

	var rootCAs *x509.CertPool
	if len(c.CA) != 0 {
		rootCAs = x509.NewCertPool()
		certPEMBlock, err := os.ReadFile(c.CA)
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
		InsecureSkipVerify: len(chb) > 0 || c.InsecureSkipVerify,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(chb) > 0 {
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

	var dialRemote func(ctx context.Context) (net.Conn, error)
	if c.GRPC {
		grpcClientConn, err := grpc.Dial(c.DstAddr,
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:                time.Second * 20,
				Timeout:             time.Second * 5,
				PermitWithoutStream: false,
			}),
			grpc.WithInitialWindowSize(64*1024),
			grpc.WithInitialConnWindowSize(64*1024),
			grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
			grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
				remoteConn, err := dialer.DialContext(ctx, "tcp", s)
				if remoteConn != nil {
					applyTCPSocketBuf(remoteConn, c.OutboundBuf)
				}
				return remoteConn, err
			}))
		if err != nil {
			return fmt.Errorf("failed to init grpc conn, %w", err)
		}
		grpcClient := grpc_tunnel.NewGRPCTunnelClient(grpcClientConn)
		dialRemote = func(_ context.Context) (net.Conn, error) {
			ctx := context.Background()
			if len(c.GRPCAuth) > 0 {
				ctx = metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{grpcAuthHeader: c.GRPCAuth}))
			}
			stream, err := grpcClient.Connect(ctx)
			if err != nil {
				return nil, err
			}
			return newGrpcPeerConn(stream), nil
		}
	} else {
		dialRemote = func(ctx context.Context) (net.Conn, error) {
			tlsDialer := tls.Dialer{NetDialer: dialer, Config: tlsConfig}
			remoteConn, err := tlsDialer.DialContext(ctx, "tcp", c.DstAddr)
			if remoteConn != nil {
				applyTCPSocketBuf(remoteConn, c.OutboundBuf)
			}
			return remoteConn, err
		}
	}

	for {
		clientConn, err := l.Accept()
		if err != nil {
			return err
		}

		go func() {
			defer clientConn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			serverConn, err := dialRemote(ctx)
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

type listenerWrapper struct {
	buf    int
	innerL net.Listener
}

func wrapListener(l net.Listener, buf int) net.Listener {
	lw, ok := l.(*listenerWrapper)
	if ok {
		lw.buf = buf
		return lw
	}
	return &listenerWrapper{
		buf:    buf,
		innerL: l,
	}
}

func (l *listenerWrapper) Accept() (net.Conn, error) {
	c, err := l.innerL.Accept()
	if c != nil {
		applyTCPSocketBuf(c, l.buf)
	}
	return c, err
}

func (l *listenerWrapper) Close() error {
	return l.innerL.Close()
}

func (l *listenerWrapper) Addr() net.Addr {
	return l.innerL.Addr()
}
