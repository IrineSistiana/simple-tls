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
	"github.com/IrineSistiana/simple-tls/core/mlog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

var logger = mlog.L()

func main() {
	go func() {
		// wait signals
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		s := <-osSignals
		logger.Info("exiting on signal", zap.Stringer("signal", s))
		os.Exit(0)
	}()

	var bindAddr, dstAddr, grpcPath, serverName, ca, cert, key, hashCert, certHash, template string
	var insecureSkipVerify, isServer, vpn, genCert, showVersion, grpc, debug bool
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
	commandLine.BoolVar(&debug, "vv", false, "verbose log")

	err := commandLine.Parse(os.Args[1:])
	if err != nil {
		logger.Fatal("invalid arg", zap.Error(err))
	}

	if debug {
		mlog.SetLvl(zapcore.DebugLevel)
	}

	// display version
	if showVersion {
		println(version)
		return
	}

	// gen cert
	if genCert {
		logger.Info("generating PEM encoded key and cert")

		var templateCert *x509.Certificate
		if len(template) > 0 {
			templateCert, err = core.LoadCert(template)
			if err != nil {
				logger.Fatal("cannot load template certificate", zap.Error(err))
			}
		}
		dnsName, certOut, keyPEM, certPEM, err := core.GenerateCertificate(serverName, templateCert)
		if err != nil {
			logger.Fatal("generateCertificate", zap.Error(err))
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
		logger.Info("generating PEM encoded key", zap.String("file", key))
		keyFile, err := os.Create(key)
		if err != nil {
			logger.Fatal("creating key file", zap.String("file", key), zap.Error(err))
		}
		defer keyFile.Close()

		_, err = keyFile.Write(keyPEM)
		if err != nil {
			logger.Fatal("writing key file", zap.String("file", key), zap.Error(err))
		}

		// cert
		if len(cert) == 0 {
			cert = defaultOutFileName + ".cert"
		}
		logger.Info("generating PEM encoded cert", zap.String("file", cert))
		certFile, err := os.Create(cert)
		if err != nil {
			logger.Fatal("creating cert file", zap.String("file", cert), zap.Error(err))
		}
		defer certFile.Close()
		_, err = certFile.Write(certPEM)
		if err != nil {
			logger.Fatal("writing cert file", zap.String("file", cert), zap.Error(err))
		}

		fmt.Print("Your new cert hash is:\n")
		fmt.Printf("%x\n", sha256.Sum256(certOut.RawTBSCertificate))
		return
	}

	if len(hashCert) != 0 {
		rawCert, err := os.ReadFile(hashCert)
		if err != nil {
			logger.Fatal("failed to read cert file", zap.Error(err))
		}
		b, _ := pem.Decode(rawCert)
		if b.Type != "CERTIFICATE" {
			logger.Fatal("invaild pem type", zap.String("type", b.Type))
		}

		certs, err := x509.ParseCertificates(b.Bytes)
		if err != nil {
			logger.Fatal("failed to parse cert file", zap.Error(err))
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
		logger.Fatal("sip003 error", zap.Error(err))
	}
	if sip003Args != nil {
		logger.Info("simple-tls is running as a sip003 plugin")

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
					logger.Fatal("invalid numeric value of sip003 key", zap.String("key", key), zap.Error(err))
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
			logger.Info("running in a android vpn mode")
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
		logger.Fatal("bind addr is required")
	}
	if len(dstAddr) == 0 {
		logger.Fatal("destination addr is required")
	}

	logger.Info(
		"simple-tls is starting",
		zap.String("version", version),
		zap.String("go_version", runtime.Version()),
		zap.String("os", runtime.GOOS),
		zap.String("arch", runtime.GOARCH),
	)

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
			logger.Fatal("server exited", zap.Error(err))
		}
		logger.Info("server exited")
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
			logger.Fatal("client exited", zap.Error(err))
		}
		logger.Info("client exited")
		return
	}
}
