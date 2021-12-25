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
	"io/ioutil"
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
		//wait signals
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		s := <-osSignals
		log.Printf("exiting: signal: %v", s)
		os.Exit(0)
	}()

	var bindAddr, dstAddr, auth, serverName, wsPath, ca, cert, key, hashCert, certHash, template string
	var ws, insecureSkipVerify, isServer, noTLS, tfo, vpn, genCert, showVersion bool
	var cpu, mux, ttl int
	var timeout time.Duration
	var timeoutFlag int

	commandLine := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	commandLine.StringVar(&bindAddr, "b", "", "[Host:Port] bind address")
	commandLine.StringVar(&dstAddr, "d", "", "[Host:Port] destination address")
	commandLine.StringVar(&auth, "auth", "", "server password")

	commandLine.BoolVar(&ws, "ws", false, "websocket mode")
	commandLine.StringVar(&wsPath, "ws-path", "", "websocket path")

	// client only
	commandLine.IntVar(&mux, "mux", 0, "enable mux")
	commandLine.StringVar(&serverName, "n", "", "server name")
	commandLine.StringVar(&ca, "ca", "", "PEM CA file path")
	commandLine.StringVar(&certHash, "cert-hash", "", "server certificate hash")

	commandLine.BoolVar(&insecureSkipVerify, "no-verify", false, "client won't verify the server's certificate chain and host name")
	commandLine.BoolVar(&vpn, "V", false, "DO NOT USE, this is for android vpn mode")

	// server only
	commandLine.BoolVar(&isServer, "s", false, "run as a server (without this simple-tls runs as a client)")
	commandLine.StringVar(&cert, "cert", "", "PEM cert file")
	commandLine.StringVar(&key, "key", "", "PEM key file")
	commandLine.BoolVar(&noTLS, "no-tls", false, "disable server tls")

	// etc
	commandLine.IntVar(&timeoutFlag, "t", 300, "timeout in sec")
	commandLine.BoolVar(&tfo, "fast-open", false, "enable tfo, only available on linux 4.11+")
	commandLine.IntVar(&ttl, "ttl", 0, "set the time-to-live field that is sed in every packet.")
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
		rawCert, err := ioutil.ReadFile(hashCert)
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

		_, ok = sip003Args.SS_PLUGIN_OPTIONS["ws"]
		ws = ws || ok
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["ws-path"]
		setStrIfNotEmpty(&wsPath, s)

		// client
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["n"]
		setStrIfNotEmpty(&serverName, s)
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["mux"]
		if err := setIntIfNotZero(&mux, s); err != nil {
			log.Fatalf("invalid mux value, %v", err)
		}
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["ca"]
		setStrIfNotEmpty(&ca, s)
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["cert-hash"]
		setStrIfNotEmpty(&certHash, s)

		_, ok = sip003Args.SS_PLUGIN_OPTIONS["no-verify"]
		insecureSkipVerify = insecureSkipVerify || ok

		// server
		_, ok = sip003Args.SS_PLUGIN_OPTIONS["s"]
		isServer = isServer || ok
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["cert"]
		setStrIfNotEmpty(&cert, s)
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["key"]
		setStrIfNotEmpty(&key, s)
		_, ok = sip003Args.SS_PLUGIN_OPTIONS["no-tls"]
		noTLS = noTLS || ok

		// etc
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["t"]
		if err := setIntIfNotZero(&timeoutFlag, s); err != nil {
			log.Fatalf("invalid timeout value, %v", err)
		}
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["cpu"]
		if err := setIntIfNotZero(&cpu, s); err != nil {
			log.Fatalf("invalid cpu number, %v", err)
		}
		_, ok = sip003Args.SS_PLUGIN_OPTIONS["fast-open"]
		tfo = tfo || ok
		s, _ = sip003Args.SS_PLUGIN_OPTIONS["ttl"]
		if err := setIntIfNotZero(&ttl, s); err != nil {
			log.Fatalf("invalid ttl number, %v", err)
		}

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
			BindAddr:      bindAddr,
			DstAddr:       dstAddr,
			Websocket:     ws,
			WebsocketPath: wsPath,
			Cert:          cert,
			Key:           key,
			ServerName:    serverName,
			Auth:          auth,
			TFO:           tfo,
			IdleTimeout:   timeout,
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
			Websocket:          ws,
			WebsocketPath:      wsPath,
			Mux:                mux,
			Auth:               auth,
			ServerName:         serverName,
			CA:                 ca,
			CertHash:           certHash,
			InsecureSkipVerify: insecureSkipVerify,
			IdleTimeout:        timeout,
			SocketOpts: &core.TcpConfig{
				AndroidVPN: vpn,
				EnableTFO:  tfo,
				TTL:        ttl,
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
