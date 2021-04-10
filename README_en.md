# simple-tls

[中文](README.md) [English](README_en.md)

---

- [simple-tls](#simple-tls)
  - [Usage](#usage)
  - [SIP003 mode](#sip003-mode)
  - [Android](#android)

## Usage

```text
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
  -auth string
      Authentication password. (Optional. The client and server must be the same. Only used to filter scan traffic.)

# Client arguments
# e.g. simple-tls -b 127.0.0.1:1080 -d your_server_ip:1080 -n your.server.name

  -mux int
      The maximum number of multiplexed connections in a single TCP connection. (Default 0 disables mux)
  -n string
      Server certificate name.
  -no-verify
      Client won't verify the server's certificate chain and host name.
  -ca string
      Load a CA file from path.
      e.g. -ca ./path/to/my.ca.cert
  -cca string
      Load a base64 encoded (e.g. base64 -w 0 ./my.cert) PEM CA certificate from string. 
      e.g. -cca VkRJW...4eGdFbz0K==

# Server arguments
# e.g. simple-tls -b 0.0.0.0:1080 -d 127.0.0.1:12345 -s -key /path/to/your/key -cert /path/to/your/cert
# -cert and -key can be left blank, a temporary certificate will be generated. The Subject Alternate Name of the certificate is taken from the `-n` parameter.
# e.g. simple-tls -b 0.0.0.0:1080 -d 127.0.0.1:12345 -s -n my.test.domain

  -s    
      (Required) Run as a server.
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
      Quickly generate an ECC certificate with Subject Alternate Name taken from the `-n` parameter.
      e.g. simple-tls -gen-cert -n my.test.domain
  -v
      Print out version information of the current binary.
```

## SIP003 mode

Complies with Shadowsocks [SIP003](https://shadowsocks.org/en/wiki/Plugin.html) plugin protocol. Shadowsocks will automatically set `-d` and `-b` parameters, no need to set those manually.

```shell
ss-server -c config.json --plugin simple-tls --plugin-opts "s;key=/path/to/your/key;cert=/path/to/your/cert"
ss-local -c config.json --plugin simple-tls --plugin-opts "n=your.server.certificates.dnsname"
```

## Android

simple-tls-android is a GUI plugin for [shadowsocks-android](https://github.com/shadowsocks/shadowsocks-android). You need to download and install shadowsocks-android first. It's also an open source software. Source code is available [here](https://github.com/IrineSistiana/simple-tls-android).

<details><summary><code>Screenshot</code></summary>

<br>

![screenshot](/assets/simple-tls-android-screenshot.jpg)

</details>

---
