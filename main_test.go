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

package main

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
	test := func(sendPaddingData bool) {
		// start server
		_, keyPEM, certPEM, err := generateCertificate("example.com")
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			t.Fatal(err)
		}

		serverListener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer serverListener.Close()

		go doServer(serverListener, []tls.Certificate{cert}, echoListener.Addr().String(), sendPaddingData, timeout)

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

		go doClient(clientListener, serverListener.Addr().String(), "example.com", caPool, sendPaddingData, timeout, false, false)

		log.Printf("echo: %v, server: %v client: %v", echoListener.Addr(), serverListener.Addr(), clientListener.Addr())
		conn, err := net.Dial("tcp", clientListener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		data := randData()
		buf := make([]byte, dataSize)

		for i := 0; i < 10; i++ {
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

	// test tls
	t.Log("testing tls")
	test(false)

	// test tls with random header
	t.Log("testing tls with random header")
	test(true)
}
