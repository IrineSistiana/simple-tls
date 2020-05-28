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
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func main() {
	log.Print("main: simple-tls")
	go func() {
		//wait signals
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		s := <-osSignals
		log.Printf("main: exiting: signal: %v", s)
		os.Exit(0)
	}()

	var bindAddr, dstAddr, serverName, cca, cert, key, path string
	var isServer, wss, sendRandomHeader, tfo, vpn, genCert bool
	var cpu int
	var timeout time.Duration
	var timeoutFlag int

	commandLine := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	commandLine.StringVar(&bindAddr, "b", "", "[Host:Port] bind address")
	commandLine.StringVar(&dstAddr, "d", "", "[Host:Port] destination address")
	commandLine.BoolVar(&wss, "wss", false, "using wss protocol")
	commandLine.StringVar(&path, "path", "/", "[path] wss path")
	commandLine.BoolVar(&sendRandomHeader, "rh", false, "add a random header to every connection to against traffic analysis")

	// client only
	commandLine.StringVar(&serverName, "n", "", "server name")
	commandLine.StringVar(&cca, "cca", "", "base64 encoded PEM CA. Client will use it to varify the server")

	// server only
	commandLine.BoolVar(&isServer, "s", false, "is server")
	commandLine.StringVar(&cert, "cert", "", "[Path] PEM cert file")
	commandLine.StringVar(&key, "key", "", "[Path] PEM key file")

	// etc
	commandLine.IntVar(&timeoutFlag, "t", 300, "timeout after sec")
	commandLine.BoolVar(&tfo, "fast-open", false, "enable tfo, only available on linux 4.11+")
	commandLine.IntVar(&cpu, "cpu", runtime.NumCPU(), "the maximum number of CPUs that can be executing simultaneously")

	commandLine.BoolVar(&genCert, "gen-cert", false, "[This is a helper function]: generate a certificate, store it's key to [-key] and cert to [-cert], print cert in base64 format without padding characters")

	sip003Args, err := GetSIP003Args()
	if err != nil {
		log.Fatalf("main: sip003 error: %v", err)
	}

	// overwrite args from env
	if sip003Args != nil {
		log.Print("main: simple-tls is running as a sip003 plugin")

		opts, err := FormatSSPluginOptions(sip003Args.SS_PLUGIN_OPTIONS)
		if err != nil {
			log.Fatalf("main: invalid sip003 SS_PLUGIN_OPTIONS: %v", err)
		}

		if err := commandLine.Parse(opts); err != nil {
			log.Printf("main: WARNING: sip003Args: commandLine.Parse: %v", err)
		}

		if isServer {
			dstAddr = sip003Args.GetLocalAddr()
			bindAddr = sip003Args.GetRemoteAddr()
		} else {
			bindAddr = sip003Args.GetLocalAddr()
			dstAddr = sip003Args.GetRemoteAddr()
		}
		tfo = sip003Args.TFO
		vpn = sip003Args.VPN

	} else {
		err := commandLine.Parse(os.Args[1:])
		if err != nil {
			log.Fatalf("main: commandLine.Parse: %v", err)
		}
	}

	// gen cert
	if genCert {
		log.Print("main: WARNNING: generating PEM encoded key and cert")

		dnsName, keyPEM, certPEM, err := generateCertificate(serverName)
		if err != nil {
			log.Fatalf("main: generateCertificate: %v", err)
		}

		// key
		if len(key) == 0 {
			key = dnsName + ".key"
		}
		log.Printf("main: generating PEM encoded key to %s", key)
		keyFile, err := os.Create(key)
		if err != nil {
			log.Fatalf("main: creating key file [%s]: %v", key, err)
		}
		defer keyFile.Close()

		_, err = keyFile.Write(keyPEM)
		if err != nil {
			log.Fatalf("main: writing key file [%s]: %v", key, err)
		}

		// cert
		if len(cert) == 0 {
			cert = dnsName + ".cert"
		}
		log.Printf("main: generating PEM encoded cert to %s", cert)
		certFile, err := os.Create(cert)
		if err != nil {
			log.Fatalf("main: creating cert file [%s]: %v", cert, err)
		}
		defer certFile.Close()
		_, err = certFile.Write(certPEM)
		if err != nil {
			log.Fatalf("main: writing cert file [%s]: %v", cert, err)
		}

		certBase64 := base64.RawStdEncoding.EncodeToString(certPEM)
		fmt.Printf("Your new cert dns name is: %s\n", dnsName)
		fmt.Print("Your new cert base64 string is:\n")
		fmt.Printf("%s\n", certBase64)
		fmt.Println("Copy this string and import it to client using -cca option")
		return
	}

	timeout = time.Duration(timeoutFlag) * time.Second
	runtime.GOMAXPROCS(cpu)

	if len(bindAddr) == 0 {
		log.Fatal("main: bind addr is required")
	}
	if len(dstAddr) == 0 {
		log.Fatal("main: destination addr is required")
	}

	if isServer {
		tlsConfig := new(tls.Config)
		tlsConfig.MinVersion = tls.VersionTLS13
		if len(cert) == 0 || len(key) == 0 {
			log.Fatal("main: server must have a X509 key pair, aka. -cert and -key")
		} else {
			cer, err := tls.LoadX509KeyPair(cert, key) //load cert
			if err != nil {
				log.Fatalf("main: LoadX509KeyPair: %v", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cer}
		}

		lc := net.ListenConfig{Control: getControlFunc(&tcpConfig{tfo: tfo})}
		l, err := lc.Listen(context.Background(), "tcp", bindAddr)
		if err != nil {
			log.Fatalf("main: net.Listen: %v", err)
		}

		err = doServer(l, tlsConfig, dstAddr, wss, path, sendRandomHeader, timeout)
		if err != nil {
			log.Fatalf("main: doServer: %v", err)
		}

	} else { // do client
		tlsConfig := new(tls.Config)
		tlsConfig.ClientSessionCache = tls.NewLRUClientSessionCache(8)
		tlsConfig.MinVersion = tls.VersionTLS13
		var host string
		if len(serverName) != 0 {
			host = serverName
		} else {
			host = strings.SplitN(bindAddr, ":", 2)[0]
		}
		if len(cca) != 0 {
			pem, err := base64.RawStdEncoding.DecodeString(cca)
			if err != nil {
				log.Fatalf("main: base64.StdEncoding.DecodeString: %v", err)
			}

			rootCAs := x509.NewCertPool()
			if ok := rootCAs.AppendCertsFromPEM(pem); !ok {
				log.Fatal("main: AppendCertsFromPEM failed, cca is invaild")
			}
			tlsConfig.RootCAs = rootCAs
		}

		lc := net.ListenConfig{}
		l, err := lc.Listen(context.Background(), "tcp", bindAddr)
		if err != nil {
			log.Fatalf("main: net.Listen: %v", err)
		}

		err = doClient(l, dstAddr, host, tlsConfig, wss, path, sendRandomHeader, timeout, vpn, tfo)
		if err != nil {
			log.Fatalf("main: doServer: %v", err)
		}
	}
}
