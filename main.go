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
	"strings"
	"syscall"
	"time"

	"github.com/IrineSistiana/simple-tls/core"
)

var version = "unknown/dev"

func main() {
	log.Printf("main: simple-tls %s", version)
	go func() {
		//wait signals
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		s := <-osSignals
		log.Printf("main: exiting: signal: %v", s)
		os.Exit(0)
	}()

	var bindAddr, dstAddr, serverName, cca, ca, cert, key string
	var insecureSkipVerify, isServer, sendPaddingData, tfo, vpn, genCert, showVersion bool
	var cpu int
	var timeout time.Duration
	var timeoutFlag int

	commandLine := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	commandLine.StringVar(&bindAddr, "b", "", "[Host:Port] bind address")
	commandLine.StringVar(&dstAddr, "d", "", "[Host:Port] destination address")
	commandLine.BoolVar(&sendPaddingData, "pd", false, "send padding data occasionally to against traffic analysis")

	// client only
	commandLine.StringVar(&serverName, "n", "", "server name")
	commandLine.StringVar(&ca, "ca", "", "PEM CA file path")
	commandLine.StringVar(&cca, "cca", "", "base64 encoded PEM CA")
	commandLine.BoolVar(&insecureSkipVerify, "no-verify", false, "client won't verify the server's certificate chain and host name")
	commandLine.BoolVar(&vpn, "V", false, "enable android vpn mode.(DO NOT USE. Only for shadowsocks-android)")

	// server only
	commandLine.BoolVar(&isServer, "s", false, "is server")
	commandLine.StringVar(&cert, "cert", "", "[Path] PEM cert file")
	commandLine.StringVar(&key, "key", "", "[Path] PEM key file")

	// etc
	commandLine.IntVar(&timeoutFlag, "t", 300, "timeout after sec")
	commandLine.BoolVar(&tfo, "fast-open", false, "enable tfo, only available on linux 4.11+")
	commandLine.IntVar(&cpu, "cpu", runtime.NumCPU(), "the maximum number of CPUs that can be executing simultaneously")

	commandLine.BoolVar(&genCert, "gen-cert", false, "[This is a helper function]: generate a certificate, store it's key to [-key] and cert to [-cert], print cert in base64 format without padding characters")
	commandLine.BoolVar(&showVersion, "v", false, "output version info and exit")

	sip003Args, err := core.GetSIP003Args()
	if err != nil {
		log.Fatalf("main: sip003 error: %v", err)
	}

	// overwrite args from env
	if sip003Args != nil {
		log.Print("main: simple-tls is running as a sip003 plugin")

		opts, err := core.FormatSSPluginOptions(sip003Args.SS_PLUGIN_OPTIONS)
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
		tfo = tfo || sip003Args.TFO
		vpn = vpn || sip003Args.VPN

	} else {
		err := commandLine.Parse(os.Args[1:])
		if err != nil {
			log.Fatalf("main: commandLine.Parse: %v", err)
		}
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

	timeout = time.Duration(timeoutFlag) * time.Second
	runtime.GOMAXPROCS(cpu)

	if len(bindAddr) == 0 {
		log.Fatal("main: bind addr is required")
	}
	if len(dstAddr) == 0 {
		log.Fatal("main: destination addr is required")
	}

	if isServer {
		var certificates []tls.Certificate

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

		lc := net.ListenConfig{Control: core.GetControlFunc(&core.TcpConfig{EnableTFO: tfo})}
		l, err := lc.Listen(context.Background(), "tcp", bindAddr)
		if err != nil {
			log.Fatalf("main: net.Listen: %v", err)
		}

		err = core.DoServer(l, certificates, dstAddr, sendPaddingData, timeout)
		if err != nil {
			log.Fatalf("main: doServer: %v", err)
		}

	} else { // do client
		var host string
		if len(serverName) != 0 {
			host = serverName
		} else {
			host = strings.SplitN(bindAddr, ":", 2)[0]
		}
		var rootCAs *x509.CertPool

		switch {
		case len(cca) != 0:
			cca = strings.TrimRight(cca, "=")
			pem, err := base64.RawStdEncoding.DecodeString(cca)
			if err != nil {
				log.Fatalf("main: base64.RawStdEncoding.DecodeString: %v", err)
			}

			rootCAs = x509.NewCertPool()
			if ok := rootCAs.AppendCertsFromPEM(pem); !ok {
				log.Fatal("main: AppendCertsFromPEM failed, cca is invaild")
			}
		case len(ca) != 0:
			rootCAs = x509.NewCertPool()
			certPEMBlock, err := ioutil.ReadFile(ca)
			if err != nil {
				log.Fatalf("main: ReadFile ca [%s], %v", ca, err)
			}
			if ok := rootCAs.AppendCertsFromPEM(certPEMBlock); !ok {
				log.Fatal("main: AppendCertsFromPEM failed, ca is invaild")
			}
		}

		lc := net.ListenConfig{}
		l, err := lc.Listen(context.Background(), "tcp", bindAddr)
		if err != nil {
			log.Fatalf("main: net.Listen: %v", err)
		}

		err = core.DoClient(l, dstAddr, host, rootCAs, insecureSkipVerify, sendPaddingData, timeout, vpn, tfo)
		if err != nil {
			log.Fatalf("main: doServer: %v", err)
		}
	}
}
