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

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/IrineSistiana/simple-tls/core"
)

var version = "unknown/dev"

func main() {
	go func() {
		//wait signals
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		s := <-osSignals
		log.Printf("main: exiting: signal: %v", s)
		os.Exit(0)
	}()

	var bindAddr, dstAddr, auth, serverName, cca, ca, cert, key string
	var noTLS, insecureSkipVerify, isServer, tfo, vpn, genCert, showVersion bool
	var cpu, mux int
	var timeout time.Duration
	var timeoutFlag int

	commandLine := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	commandLine.StringVar(&bindAddr, "b", "", "[Host:Port] bind address")
	commandLine.StringVar(&dstAddr, "d", "", "[Host:Port] destination address")
	commandLine.StringVar(&auth, "auth", "", "server password")
	commandLine.BoolVar(&noTLS, "no-tls", false, "disable TLS (debug only)")

	// client only
	commandLine.IntVar(&mux, "mux", 0, "enable mux")
	commandLine.StringVar(&serverName, "n", "", "server name")
	commandLine.StringVar(&ca, "ca", "", "PEM CA file path")
	commandLine.StringVar(&cca, "cca", "", "base64 encoded PEM CA")
	commandLine.BoolVar(&insecureSkipVerify, "no-verify", false, "client won't verify the server's certificate chain and host name")
	commandLine.BoolVar(&vpn, "V", false, "DO NOT USE, this is for android vpn mode")

	// server only
	commandLine.BoolVar(&isServer, "s", false, "is server")
	commandLine.StringVar(&cert, "cert", "", "[Path] PEM cert file")
	commandLine.StringVar(&key, "key", "", "[Path] PEM key file")

	// etc
	commandLine.IntVar(&timeoutFlag, "t", 300, "timeout in sec")
	commandLine.BoolVar(&tfo, "fast-open", false, "enable tfo, only available on linux 4.11+")
	commandLine.IntVar(&cpu, "cpu", runtime.NumCPU(), "the maximum number of CPUs that can be executing simultaneously")

	// helper commands
	commandLine.BoolVar(&genCert, "gen-cert", false, "[This is a helper function]: generate a certificate with dns name [-n], store it's key to [-key] and cert to [-cert], print cert in base64 format without padding characters")
	commandLine.BoolVar(&showVersion, "v", false, "output version info and exit")

	err := commandLine.Parse(os.Args[1:])
	if err != nil {
		log.Fatalf("main: invalid arg: %v", err)
	}

	// display version
	if showVersion {
		println(version)
		os.Exit(0)
	}

	// gen cert
	if genCert {
		log.Print("main: WARNING: generating PEM encoded key and cert")

		dnsName, keyPEM, certPEM, err := core.GenerateCertificate(serverName)
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

	// overwrite args from env
	sip003Args, err := core.GetSIP003Args()
	if err != nil {
		log.Fatalf("main: sip003 error: %v", err)
	}
	if sip003Args != nil {
		log.Print("main: simple-tls is running as a sip003 plugin")

		var ok bool
		var s string

		// android only
		_, ok = sip003Args.SS_PLUGIN_OPTIONS["V"]
		vpn = vpn || ok
		_, ok = sip003Args.SS_PLUGIN_OPTIONS["__android_vpn"]
		vpn = vpn || ok

		// common
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["b"]
		setStrIfNotEmpty(&bindAddr, s)
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["d"]
		setStrIfNotEmpty(&dstAddr, s)
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["auth"]
		setStrIfNotEmpty(&auth, s)
		_, ok = sip003Args.SS_PLUGIN_OPTIONS["no-tls"]
		noTLS = noTLS || ok

		// client
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["n"]
		setStrIfNotEmpty(&serverName, s)
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["mux"]
		if err := setIntIfNotZero(&mux, s); err != nil {
			log.Fatalf("main: invalid mux value, %v", err)
		}
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["ca"]
		setStrIfNotEmpty(&ca, s)
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["cca"]
		setStrIfNotEmpty(&cca, s)
		_, ok = sip003Args.SS_PLUGIN_OPTIONS["no-verify"]
		insecureSkipVerify = insecureSkipVerify || ok

		// server
		_, ok = sip003Args.SS_PLUGIN_OPTIONS["s"]
		isServer = isServer || ok
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["cert"]
		setStrIfNotEmpty(&cert, s)
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["key"]
		setStrIfNotEmpty(&key, s)

		// etc
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["t"]
		if err := setIntIfNotZero(&timeoutFlag, s); err != nil {
			log.Fatalf("main: invalid timeout value, %v", err)
		}
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["cpu"]
		if err := setIntIfNotZero(&cpu, s); err != nil {
			log.Fatalf("main: invalid cpu number, %v", err)
		}
		_, ok = sip003Args.SS_PLUGIN_OPTIONS["fast-open"]
		tfo = tfo || ok

		if isServer {
			dstAddr = sip003Args.GetLocalAddr()
			bindAddr = sip003Args.GetRemoteAddr()
		} else {
			bindAddr = sip003Args.GetLocalAddr()
			dstAddr = sip003Args.GetRemoteAddr()
		}
	}

	timeout = time.Duration(timeoutFlag) * time.Second
	runtime.GOMAXPROCS(cpu)

	if len(bindAddr) == 0 {
		log.Fatal("main: bind addr is required")
	}
	if len(dstAddr) == 0 {
		log.Fatal("main: destination addr is required")
	}

	log.Printf("main: simple-tls %s (go version: %s, os: %s, arch: %s)", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)

	if isServer {
		var certificates []tls.Certificate
		if !noTLS {
			switch {
			case len(cert) == 0 && len(key) == 0: // no cert and key
				log.Printf("main: warnning: neither -key nor -cert is specified")

				dnsName, keyPEM, certPEM, err := core.GenerateCertificate(serverName)
				if err != nil {
					log.Fatalf("main: generateCertificate: %v", err)
				}
				log.Printf("main: warnning: using tmp certificate %s", dnsName)
				cer, err := tls.X509KeyPair(certPEM, keyPEM)
				if err != nil {
					log.Fatalf("main: X509KeyPair: %v", err)
				}
				certificates = []tls.Certificate{cer}
			case len(cert) != 0 && len(key) != 0: // has cert and key
				cer, err := tls.LoadX509KeyPair(cert, key) //load cert
				if err != nil {
					log.Fatalf("main: LoadX509KeyPair: %v", err)
				}
				certificates = []tls.Certificate{cer}
			default:
				log.Fatal("main: server must have a X509 key pair, aka. -cert and -key")
			}
		}

		lc := net.ListenConfig{Control: core.GetControlFunc(&core.TcpConfig{EnableTFO: tfo})}
		l, err := lc.Listen(context.Background(), "tcp", bindAddr)
		if err != nil {
			log.Fatalf("main: net.Listen: %v", err)
		}

		server := core.Server{
			Listener:     l,
			Dst:          dstAddr,
			NoTLS:        noTLS,
			Auth:         auth,
			Certificates: certificates,
			Timeout:      timeout,
		}

		err = server.ActiveAndServe()
		if err != nil {
			log.Fatalf("main: doServer: %v", err)
		}

	} else { // do client
		var rootCAs *x509.CertPool
		if !noTLS {
			if len(serverName) == 0 {
				serverName = strings.SplitN(dstAddr, ":", 2)[0]
			}

			switch {
			case len(cca) != 0:
				cca = strings.TrimRight(cca, "=")
				pem, err := base64.RawStdEncoding.DecodeString(cca)
				if err != nil {
					log.Fatalf("main: base64.RawStdEncoding.DecodeString: %v", err)
				}

				rootCAs = x509.NewCertPool()
				if ok := rootCAs.AppendCertsFromPEM(pem); !ok {
					log.Fatal("main: AppendCertsFromPEM failed, cca is invalid")
				}
			case len(ca) != 0:
				rootCAs = x509.NewCertPool()
				certPEMBlock, err := ioutil.ReadFile(ca)
				if err != nil {
					log.Fatalf("main: ReadFile ca [%s], %v", ca, err)
				}
				if ok := rootCAs.AppendCertsFromPEM(certPEMBlock); !ok {
					log.Fatal("main: AppendCertsFromPEM failed, ca is invalid")
				}
			}
		}

		lc := net.ListenConfig{}
		l, err := lc.Listen(context.Background(), "tcp", bindAddr)
		if err != nil {
			log.Fatalf("main: net.Listen: %v", err)
		}

		client := core.Client{
			Listener:           l,
			ServerAddr:         dstAddr,
			NoTLS:              noTLS,
			Auth:               auth,
			ServerName:         serverName,
			CertPool:           rootCAs,
			InsecureSkipVerify: insecureSkipVerify,
			Timeout:            timeout,
			AndroidVPNMode:     vpn,
			TFO:                tfo,
			Mux:                mux,
		}

		err = client.ActiveAndServe()
		if err != nil {
			log.Fatalf("main: doServer: %v", err)
		}
	}
}

func setStrIfNotEmpty(dst *string, src string) {
	if len(src) != 0 {
		*dst = src
	}
}

func setIntIfNotZero(dst *int, src string) error {
	if len(src) != 0 {
		i, err := strconv.Atoi(src)
		if err != nil {
			return fmt.Errorf("string %s is not an int", src)
		}
		if i > 0 {
			*dst = i
		}
	}
	return nil
}
