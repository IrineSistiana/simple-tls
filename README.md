# simple-tls

[中文](README_zh.md) [English](README.md)

---

Probably the simplest TLS plugin. It can:

- Protect and obfuscate your connections with real TLS1.3.
- Easily run as a SIP003 plugin and run on Android platform.
- Send random data packets at appropriate times. This can change the timing characteristics of data packets in one connection, which can protect you against timing traffic analysis. (optional, experimental) See [documentation](https://github.com/IrineSistiana/simple-tls/wiki/%E6%97%B6%E5%BA%8F%E5%A1%AB%E5%85%85(pd)%E6%A8%A1%E5%BC%8F) (Chinese only).

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
        [Host:Port] bind addr.
    -d string
        [Host:Port] destination addr.

    # Transfer mode (Client and server must have the same mode)
    -pd
        Enable padding-data mode. Server will send some padding data to protect against traffic analysis.

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
        PEM certificate file path.
    -key string
        PEM key file path.

    # Other geek's arguments
    -cpu int
        The maximum number of CPUs to simultaneously use.
    -fast-open
        Enable TCP-Fast-Open. Only available on Linux kernel 4.11+.
    -t int
        Idle timeout in seconds (default to 300).

    # Helper commands
    -gen-cert
        Quickly generate an ECC certificate.
    -v
        Print out version information of the current binary.

## Standalone mode

    # server
    simple-tls -b 0.0.0.0:1080 -d 127.0.0.1:12345 -s -key /path/to/your/key -cert /path/to/your/cert

    # client
    simple-tls -b 127.0.0.1:1080 -d your_server_ip:1080 -n your.server.certificates.dnsname

## SIP003 mode

Complies with Shadowsocks [SIP003](https://shadowsocks.org/en/wiki/Plugin.html) plugin protocol. Shadowsocks will automatically set `-d` and `-b` parameters, no need to set those manually.

Take [shadowsocks-libev](https://github.com/shadowsocks/shadowsocks-libev) as an example:

    ss-server -c config.json --plugin simple-tls --plugin-opts "s;key=/path/to/your/key;cert=/path/to/your/cert"
    ss-local -c config.json --plugin simple-tls --plugin-opts "n=your.server.certificates.dnsname"

## Start a server without certificate

You can use `-gen-cert` to quickly generate an [ECC certificate](https://www.digicert.com/faq/ecc.htm).

    simple-tls -gen-cert -n certificate.dnsname -key ./my_ecc_cert.key -cert ./my_ecc_cert.cert

Or you can just start the server without `-key` and `-cert`. Server will automatically generate a temporary certificate and store it in memory.

**Please note that:** In those cases, clients have to import the generated certificate as CA. See below. Else clients need to disable server certificate verification by using `-no-verify`. Not recommended because this is susceptible to man-in-the-middle attacks.

## How to import CA in client

You can use `-cca` or `-ca` to import a certificate or ca-bundle file as CA.

`-ca` accepts a path.

    simple-tls ... ... -ca ./path/to/my.ca.cert

`-cca` accepts a base64 encoded certificate.

    simple-tls ... ... -cca VkRJWkpCK1R1c3h...4eGdFbz0K==

## Android

simple-tls-android is a GUI plugin for [shadowsocks-android](https://github.com/shadowsocks/shadowsocks-android). You need to download and install shadowsocks-android first. It's also an open source software. Source code is available [here](https://github.com/IrineSistiana/simple-tls-android).

<details><summary><code>Screenshot</code></summary>

<br>

![screenshot](/assets/simple-tls-android-screenshot.jpg)

</details>

---
