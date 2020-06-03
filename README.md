# simple-tls

Probably the simplest tls/wss plugin. 

It can:

- Protect your connections with real TLS1.3 (not just obfuscating).
- Run as a SIP003 plugin and run on Android platform.
- Add a random header to every connection to against traffic analysis. (optional)
- Transfer your data via CDN. (optional)

---

Download here: [release](https://github.com/IrineSistiana/simple-tls/releases)

## Command help

        client bind addr               server bind addr
               |                             |
    |client|-->|simple-tls client|--TLS1.3-->|simple-tls server|-->|final destination|
                                             |                     |   
                                       client dst addr       server dst addr  

    # Common arguments
    -b string
        [Host:Port]  bind addr
    -d string
        [Host:Port]  destination addr
    -wss
        Use Websocket Secure protocol
    -path string
        Websocket path
    -rh 
        Add a random header(512b~16Kb) to every connection to against traffic analysis

    # Run as a client
    -n string
        Server certificate name
    -ca string
        PEM CA file path. [This imports a file]
    -cca string
        A base64 encoded PEM CA certificate. [This imports a base64 string]

    # Run as a server
    -s    
        Run as a server
    -cert string
        [Path] PEM certificate
    -key string
        [Path] PEM key

    # Other geek's arguments
    -gen-cert
        [This is a helper function]: generate a ecc certificate.
        The DNSName of this certificate will be [-n].
        It's key will be stored at [-key] and it's cert will be stored at [-cert].
        e.g. -gen-cert -n example.com -key ./example.com.key -cert ./example.com.cert

    -cpu int
        The maximum number of CPUs that can be executing simultaneously
    -fast-open
        Enable tfo, only available on linux kernel 4.11+
    -t int
        Idle timeout in sec (default 300)

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

## Tips for certificate and -cca argument

To start a server, the argument `-key` and `-cert` are required. Because simple-tls needs a certificate to establish real TLS1.3 connections.

For your safety, the server certificate verification in simple-tls **can't be disabled**. You need to use `-cca` or `-ca` argument to import the CA certificate in the client if you are using a self-signed certificate in server.

In the test environment, you can use `-gen-cert` in server to quickly generate an ECC certificate, and use `-cca` or `-ca` in the client to import its cert as CA.

## Tips for speed and stability

Considering that the TLS1.3 layer is sufficiently secure, a simple encryption can be used in lower-layer connections to increase speed.

In Linux system, decrease your tcp r/w memory by using `sysctl` to improve stability.

    sudo sysctl net.ipv4.tcp_rmem="4096 87380 1048576"
    sudo sysctl net.ipv4.tcp_wmem="4096 16384 1048576"

---

