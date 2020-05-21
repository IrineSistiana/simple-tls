# simple-tls

Probably the simplest tls/wss plugin.

---

Download: [release](https://github.com/IrineSistiana/simple-tls/releases)

## Command help

        client bind addr               server bind addr
               |                             |
    |client|-->|simple-tls client|--TLS1.3-->|simple-tls server|-->|final destination|
                                             |                     |   
                                       client dst addr       server dst addr  

    # Common options
    -b string
        [Host:Port]  bind addr
    -d string
        [Host:Port]  destination addr
    -wss
        Use Websocket Secure protocol, the option server and client need to be consistent 
    -path string
        Websocket path

    # Run as a client
    -n string
        Server certificate name
    -cca string
        A base64 encoded CA certificate in PEM format, used to verify the identity of the server.

    # Run as a server
    -s    
        Run as a server
    -cert string
        [Path] PEM certificate
    -key string
        [Path] PEM ket

    # Other geek's options
    -gen-cert
        [This is a helper function]: generate a certificate, 
        store it's key to [-key] and cert to [-cert],
        print cert in base64 format without padding characters
    -cpu int
        the maximum number of CPUs that can be executing simultaneously
    -fast-open
        enable tfo, only available on linux 4.11+
    -t int
        timeout after sec (default 300)

## Standalone mode

    Run as a server: 
        simple-tls -b 0.0.0.0:1080 -d 127.0.0.1:12345 -s -key /path/to/your/key -cert /path/to/your/cert
    Run as a client:
        simple-tls -b 127.0.0.1:1080 -d your_server_ip:1080 -n your.server.certificates.dnsname

## SIP003 mode

Comply with shadowsocks [SIP003](https://shadowsocks.org/en/spec/Plugin.html) plugin protocol. Accepted key-value pair are [same as above](#command). Shadowsocks will automatically set `-d` and `-b` parameters, no need to set manually.

Take [shadowsocks-libev](https://github.com/shadowsocks/shadowsocks-libev) as an example:

    # TLS
    ss-server -c config.json --plugin simple-tls --plugin-opts "s;key=/path/to/your/key;cert=/path/to/your/cert"
    ss-local -c config.json --plugin simple-tls --plugin-opts "n=your.server.certificates.dnsname"

    # WSS
    ss-server -c config.json --plugin simple-tls --plugin-opts "s;wss;key=/path/to/your/key;cert=/path/to/your/cert"
    ss-local -c config.json --plugin simple-tls --plugin-opts "wss;n=your.server.certificates.dnsname"

## Android

`simple-tls-android` is a plugin for [shadowsocks-android](https://github.com/shadowsocks/shadowsocks-android). You need to download and install shadowsocks-android first.

---

