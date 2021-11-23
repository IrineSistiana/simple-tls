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
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"
)

func Test_main(t *testing.T) {
	dataSize := 512 * 1024
	b := make([]byte, dataSize)
	rand.Read(b)
	randData := func() []byte {
		return b
	}

	timeout := time.Second * 15

	// echo server
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoListener.Close()
	go func() {
		for {
			c, err := echoListener.Accept()
			if err != nil {
				return
			}

			go func() {
				defer c.Close()
				buf := make([]byte, dataSize)

				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					c.Write(buf[:n])
				}
			}()
		}
	}()

	// test1
	test := func(t *testing.T, mux int, ws bool, wsPath string, auth string) {
		serverListener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer serverListener.Close()

		_, x509cert, keyPEM, certPEM, err := GenerateCertificate("")
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(x509cert.RawTBSCertificate)
		certHash := hex.EncodeToString(h[:])

		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			t.Fatal(err)
		}

		server := Server{
			DstAddr:       echoListener.Addr().String(),
			Websocket:     ws,
			WebsocketPath: wsPath,
			Auth:          auth,
			IdleTimeout:   timeout,
			testListener:  serverListener,
			testCert:      &cert,
		}

		go server.ActiveAndServe()

		// start client
		clientListener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer clientListener.Close()

		client := Client{
			DstAddr:            serverListener.Addr().String(),
			Websocket:          ws,
			WebsocketPath:      wsPath,
			Mux:                mux,
			Auth:               auth,
			CertHash:           certHash,
			InsecureSkipVerify: true,
			IdleTimeout:        timeout,
			testListener:       clientListener,
		}

		go client.ActiveAndServe()

		wg := new(sync.WaitGroup)
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				conn, err := net.Dial("tcp", clientListener.Addr().String())
				if err != nil {
					t.Error(err)
					return
				}
				data := randData()
				buf := make([]byte, dataSize)
				_, err = conn.Write(data)
				if err != nil {
					t.Error(err)
					return
				}

				_, err = io.ReadFull(conn, buf)
				if err != nil {
					t.Error(err)
					return
				}
				if bytes.Equal(data, buf) == false {
					t.Error("corrupted data")
					return
				}
			}()
		}
		wg.Wait()
	}

	for _, mux := range [...]int{0, 5} {
		for _, ws := range [...]bool{false, true} {
			for _, wsPath := range [...]string{"", "/123456"} {
				for _, auth := range [...]string{"", "123456"} {
					t.Run(fmt.Sprintf("mux_%v_ws_%v_wsPath_%v_auth_%v", mux, ws, wsPath, auth), func(t *testing.T) {
						test(t, mux, ws, wsPath, auth)
					})
				}
			}
		}
	}
}
