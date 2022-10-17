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
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
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
		// wait signals
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		s := <-osSignals
		log.Printf("exiting: signal: %v", s)
		os.Exit(0)
	}()

	var bindAddr, dstAddr, grpcPath, serverName, ca, cert, key, hashCert, certHash, template string
	var insecureSkipVerify, isServer, vpn, genCert, showVersion, grpc bool
	var cpu, outboundBufSize, inboundBufSize int
	var timeout time.Duration
	var timeoutFlag int

	commandLine := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	commandLine.StringVar(&bindAddr, "b", "", "[Host:Port] bind address")
	commandLine.StringVar(&dstAddr, "d", "", "[Host:Port] destination address")
	commandLine.BoolVar(&grpc, "grpc", false, "use grpc as a transport")
	commandLine.StringVar(&grpcPath, "grpc-path", "", "grpc auth header")
	commandLine.IntVar(&outboundBufSize, "outbound-buf", 0, "outbound socket buf size")
	commandLine.IntVar(&inboundBufSize, "inbound-buf", 0, "inbound socket buf size")

	// client only
	commandLine.StringVar(&serverName, "n", "", "server name")
	commandLine.StringVar(&ca, "ca", "", "PEM CA file path")
	commandLine.StringVar(&certHash, "cert-hash", "", "server certificate hash (pin server cert)")

	commandLine.BoolVar(&insecureSkipVerify, "no-verify", false, "client won't verify the server's certificate chain and host name")
	commandLine.BoolVar(&vpn, "V", false, "DO NOT USE, this is for android vpn mode")

	// server only
	commandLine.BoolVar(&isServer, "s", false, "run as a server (without this simple-tls runs as a client)")
	commandLine.StringVar(&cert, "cert", "", "PEM cert file")
	commandLine.StringVar(&key, "key", "", "PEM key file")

	// etc
	commandLine.IntVar(&timeoutFlag, "t", 300, "timeout in sec")
	commandLine.IntVar(&cpu, "cpu", runtime.NumCPU(), "the maximum number of CPUs that can be executing simultaneously")

	// helper commands
	commandLine.BoolVar(&genCert, "gen-cert", false, "generate a certificate with dns name [-n](optional or random) by using template [-template](optional), store it's key to [-key](optional or dns name) and cert to [-cert](optional or dns name)")
	commandLine.StringVar(&template, "template", "", "template certificate")
	commandLine.BoolVar(&showVersion, "v", false, "output version info and exit")
	commandLine.StringVar(&hashCert, "hash-cert", "", "print the hashes for the certificate")

	err := commandLine.Parse(os.Args[1:])
	if err != nil {
		log.Fatalf("invalid arg: %v", err)
	}

	// display version
	if showVersion {
		println(version)
		return
	}

	// gen cert
	if genCert {
		log.Print("generating PEM encoded key and cert")

		var templateCert *x509.Certificate
		if len(template) > 0 {
			templateCert, err = core.LoadCert(template)
			if err != nil {
				log.Fatalf("cannot load template certificate: %v", err)
			}
		}
		dnsName, certOut, keyPEM, certPEM, err := core.GenerateCertificate(serverName, templateCert)
		if err != nil {
			log.Fatalf("generateCertificate: %v", err)
		}

		var defaultOutFileName string
		if len(template) > 0 {
			defaultOutFileName = "gen_" + strings.TrimSuffix(filepath.Base(template), filepath.Ext(template))
		} else {
			defaultOutFileName = strings.TrimPrefix(dnsName, "*.")
		}

		// key
		if len(key) == 0 {
			key = defaultOutFileName + ".key"
		}
		log.Printf("generating PEM encoded key to %s", key)
		keyFile, err := os.Create(key)
		if err != nil {
			log.Fatalf("creating key file [%s]: %v", key, err)
		}
		defer keyFile.Close()

		_, err = keyFile.Write(keyPEM)
		if err != nil {
			log.Fatalf("writing key file [%s]: %v", key, err)
		}

		// cert
		if len(cert) == 0 {
			cert = defaultOutFileName + ".cert"
		}
		log.Printf("generating PEM encoded cert to %s", cert)
		certFile, err := os.Create(cert)
		if err != nil {
			log.Fatalf("creating cert file [%s]: %v", cert, err)
		}
		defer certFile.Close()
		_, err = certFile.Write(certPEM)
		if err != nil {
			log.Fatalf("writing cert file [%s]: %v", cert, err)
		}

		fmt.Print("Your new cert hash is:\n")
		fmt.Printf("%x\n", sha256.Sum256(certOut.RawTBSCertificate))
		return
	}

	if len(hashCert) != 0 {
		rawCert, err := os.ReadFile(hashCert)
		if err != nil {
			log.Fatalf("failed to read cert file: %v", err)
		}
		b, _ := pem.Decode(rawCert)
		if b.Type != "CERTIFICATE" {
			log.Fatalf("invaild pem type [%s]", b.Type)
		}

		certs, err := x509.ParseCertificates(b.Bytes)
		if err != nil {
			log.Fatalf("failed to parse cert file: %v", err)
		}
		for _, cert := range certs {
			h := sha256.Sum256(cert.RawTBSCertificate)
			fmt.Printf("[%v]: %x\n", cert.Subject, h)
		}
		return
	}

	// overwrite args from env
	sip003Args, err := core.GetSIP003Args()
	if err != nil {
		log.Fatalf("sip003 error: %v", err)
	}
	if sip003Args != nil {
		log.Print("simple-tls is running as a sip003 plugin")

		applyBoolOpt := func(v *bool, key string) {
			_, ok := sip003Args.SS_PLUGIN_OPTIONS[key]
			*v = *v || ok
		}

		applyStringOpt := func(v *string, key string) {
			s := sip003Args.SS_PLUGIN_OPTIONS[key]
			if len(s) != 0 {
				*v = s
			}
		}
		applyIntOpt := func(v *int, key string) {
			s := sip003Args.SS_PLUGIN_OPTIONS[key]
			if len(s) != 0 {
				i, err := strconv.Atoi(s)
				if err != nil {
					log.Fatalf("invalid value of key %s, %s", key, err)
				}
				if i > 0 {
					*v = i
				}
			}
		}

		// android only
		applyBoolOpt(&vpn, "V")             // before shadowsocks-android-plugin v2
		applyBoolOpt(&vpn, "__android_vpn") // v2

		if vpn {
			log.Print("running in a android vpn mode")
		}

		// common
		applyStringOpt(&bindAddr, "b")
		applyStringOpt(&dstAddr, "d")
		applyBoolOpt(&grpc, "grpc")
		applyStringOpt(&grpcPath, "grpc-path")

		// client
		applyStringOpt(&serverName, "n")
		applyStringOpt(&ca, "ca")
		applyStringOpt(&certHash, "cert-hash")
		applyBoolOpt(&insecureSkipVerify, "no-verify")

		// server
		applyBoolOpt(&isServer, "s")
		applyStringOpt(&cert, "cert")
		applyStringOpt(&key, "key")

		// etc
		applyIntOpt(&timeoutFlag, "t")
		applyIntOpt(&cpu, "cpu")
		applyIntOpt(&outboundBufSize, "outbound-buf")
		applyIntOpt(&inboundBufSize, "inbound-buf")

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
		log.Fatal("bind addr is required")
	}
	if len(dstAddr) == 0 {
		log.Fatal("destination addr is required")
	}

	log.Printf("simple-tls %s (go version: %s, os: %s, arch: %s)", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)

	if isServer {
		server := core.Server{
			BindAddr:        bindAddr,
			DstAddr:         dstAddr,
			Cert:            cert,
			Key:             key,
			ServerName:      serverName,
			GRPC:            grpc,
			GRPCServiceName: grpcPath,
			IdleTimeout:     timeout,
			OutboundBuf:     outboundBufSize,
			InboundBuf:      inboundBufSize,
		}
		if err := server.ActiveAndServe(); err != nil {
			log.Fatalf("server exited: %v", err)
		}
		log.Print("server exited")
		return

	} else { // do client
		client := core.Client{
			BindAddr:           bindAddr,
			DstAddr:            dstAddr,
			GRPC:               grpc,
			GRPCServiceName:    grpcPath,
			ServerName:         serverName,
			CA:                 ca,
			CertHash:           certHash,
			InsecureSkipVerify: insecureSkipVerify,
			IdleTimeout:        timeout,
			OutboundBuf:        outboundBufSize,
			InboundBuf:         inboundBufSize,
			SocketOpts: &core.TcpConfig{
				AndroidVPN: vpn,
			},
		}

		err = client.ActiveAndServe()
		if err != nil {
			log.Fatalf("client exited: %v", err)
		}
		log.Print("client exited")
		return
	}
}
