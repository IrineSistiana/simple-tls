# simple-tls

[中文](README_zh.md) [English](README.md)

---

Probably the simplest TLS plugin. It can:

- Protect and obfuscate your connections with real TLS1.3.
- Easily run as a SIP003 plugin and run on Android platform.
- Send random data packets at appropriate times. This can change the timing characteristics of data packets in one connection, which can protect you against timing traffic analysis. (optional, experimental)

---

- [simple-tls](#simple-tls)
  - [How to build](#how-to-build)
  - [Usage](#usage)
  - [Standalone mode](#standalone-mode)
  - [SIP003 mode](#sip003-mode)
  - [Start a server without certificate](#start-a-server-without-certificate)
  - [How to import CA in client](#how-to-import-ca-in-client)
  - [Android](#android)

## How to build

You will need go v1.14 or later.

    go build

## Usage

        client bind addr               server bind addr
               |                             |
    |client|-->|simple-tls client|--TLS1.3-->|simple-tls server|-->|final destination|
                                             |                     |   
                                       client dst addr       server dst addr  

    # Common arguments
    -b string
        [Host:Port] bind addr
    -d string
        [Host:Port] destination addr

    # Transfer mode (Client and server must have the same mode)
    -pd
        Enable padding-data mode. Server will send some padding data to against traffic analysis.

    # Client arguments
    -n string
        Server certificate name.
    -no-verify
        Client won't verify the server's certificate chain and host name.
    -ca string
        Load a CA file from path.
    -cca string
        Load a base64 encoded PEM CA certificate from string.

    # Server arguments
    -s    
        Run as a server.
    -cert string
        PEM certificate file path
    -key string
        PEM key file path

    # Other geek's arguments
    -cpu int
        The maximum number of CPUs that can be executing simultaneously.
    -fast-open
        Enable tfo, only available on linux kernel 4.11+.
    -t int
        Idle timeout in sec (default 300).

    # helper commands
    -gen-cert
        Quickly generate a ecc certificate.

## Standalone mode

    # server
    simple-tls -b 0.0.0.0:1080 -d 127.0.0.1:12345 -s -key /path/to/your/key -cert /path/to/your/cert

    # client
    simple-tls -b 127.0.0.1:1080 -d your_server_ip:1080 -n your.server.certificates.dnsname

## SIP003 mode

Comply with shadowsocks [SIP003](https://shadowsocks.org/en/spec/Plugin.html) plugin protocol. Shadowsocks will automatically set `-d` and `-b` parameters, no need to set those manually.

Take [shadowsocks-libev](https://github.com/shadowsocks/shadowsocks-libev) as an example:

    ss-server -c config.json --plugin simple-tls --plugin-opts "s;key=/path/to/your/key;cert=/path/to/your/cert"
    ss-local -c config.json --plugin simple-tls --plugin-opts "n=your.server.certificates.dnsname"

## Start a server without certificate

You can use `-gen-cert` to quickly generate an ECC certificate.

    simple-tls -gen-cert -n certificate.dnsname -key ./my_ecc_cert.key -cert ./my_ecc_cert.cert 

Or you can just start the server without `-key` and `-cert`. Server will automatically generate a temporary certificate and store it in memory.

**Please note that:** In those cases, client have to import generated cert as CA. See below. Or disable server certificate verify by using `-no-verify`. (not recommended, because this is susceptible to man-in-the-middle attacks.) 

## How to import CA in client

You can use `-cca` or `-ca` to import a cert or ca-bundle file as CA.

`-ca` accepts a path.

    simple-tls ... ... -ca ./my.ca.cert

`-cca` accepts a base64 encoded certificate.

    simple-tls ... ... -cca VkRJWkpCK1R1c3h...4eGdFbz0K==

## Android

simple-tls-android is a GUI plugin for [shadowsocks-android](https://github.com/shadowsocks/shadowsocks-android). You need to download and install shadowsocks-android first. It's also an open source software. Source code is available at [here](https://github.com/IrineSistiana/simple-tls-android).

<details><summary><code>Screenshot</code></summary><br>

![avatar](/assets/simple-tls-android-screenshot.jpg)

</details>

---
