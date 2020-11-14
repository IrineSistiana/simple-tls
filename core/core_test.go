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
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"math/rand"
	"net"
	"testing"
	"time"
)

func Test_main(t *testing.T) {
	dataSize := 512 * 1024
	randData := func() []byte {
		b := make([]byte, dataSize)
		rand.Read(b)
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
	test := func(t *testing.T, mux int) {
		// start server
		_, keyPEM, certPEM, err := GenerateCertificate("example.com")
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			t.Fatal(err)
		}

		serverListener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer serverListener.Close()

		server := Server{
			Listener:        serverListener,
			Dst:             echoListener.Addr().String(),
			Certificates:    []tls.Certificate{cert},
			Timeout:         timeout,
		}
		go server.ActiveAndServe()

		// start client
		clientListener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer clientListener.Close()

		caPool := x509.NewCertPool()
		ok := caPool.AppendCertsFromPEM(certPEM)
		if !ok {
			t.Fatal("appendCertsFromPEM failed")
		}

		client := Client{
			Listener:           clientListener,
			ServerAddr:         serverListener.Addr().String(),
			ServerName:         "example.com",
			CertPool:           caPool,
			InsecureSkipVerify: false,
			Mux:                mux,
			Timeout:            timeout,
			AndroidVPNMode:     false,
			TFO:                false,
		}
		go client.ActiveAndServe()

		log.Printf("echo: %v, server: %v client: %v", echoListener.Addr(), serverListener.Addr(), clientListener.Addr())

		for i := 0; i < 10; i++ {
			conn, err := net.Dial("tcp", clientListener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			data := randData()
			buf := make([]byte, dataSize)
			_, err = conn.Write(data)
			if err != nil {
				t.Fatal(err)
			}

			_, err = io.ReadFull(conn, buf)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(data, buf) == false {
				t.Fatal("corrupted data")
			}
		}
	}

	tests := []struct {
		name string
		mux  int
	}{
		{"plain", 0},
		{"mux", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test(t, tt.mux)
		})
	}
}
