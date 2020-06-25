# simple-tls

Probably the simplest tls plugin. 

It can:

- Protect and obfuscate your connections with real TLS1.3 (not just obfuscate with some fake headers).
- Run as a SIP003 plugin and run on Android platform.
- Can send padding data to against traffic analysis. (optional, experimental)

---

## How to build

You will need go v1.14 or later.

    $ go build

## Usage

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

    # Transfer mode (Client and server must have the same mode)
    -pd
        If enabled, server will send some padding data to against traffic analysis.

    # Client arguments
    -n string
        Server certificate name. If blank, it will be the host in -d.
    -no-verify
        If enabled, client won't verify the server's certificate chain and host name.
        If enabled, TLS is susceptible to man-in-the-middle attacks. 
    -ca string
        Load a CA file from path.
    -cca string
        Load a base64 encoded PEM CA certificate from string.

    # Server arguments
    -s    
        If enabled, simple-tls will run as a server.
    -cert string
        PEM certificate file path
    -key string
        PEM key file path

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

Comply with shadowsocks [SIP003](https://shadowsocks.org/en/spec/Plugin.html) plugin protocol. Accepted key-value pair are [same as above](#usage). Shadowsocks will automatically set `-d` and `-b` parameters, no need to set manually.

Take [shadowsocks-libev](https://github.com/shadowsocks/shadowsocks-libev) as an example:

    ss-server -c config.json --plugin simple-tls --plugin-opts "s;key=/path/to/your/key;cert=/path/to/your/cert"
    ss-local -c config.json --plugin simple-tls --plugin-opts "n=your.server.certificates.dnsname"

## Android

`simple-tls-android` is a plugin for [shadowsocks-android](https://github.com/shadowsocks/shadowsocks-android). You need to download and install shadowsocks-android first. It's [open-source](https://github.com/IrineSistiana/simple-tls-android).

## More tips

- You can start the server without `-key` and `-cert`. Server will automatically generate a temporary certificate. In this case, client have to disable verify by using  `-no-verify`.
- You can use `-gen-cert` to quickly generate an ECC certificate, and use `-cca` or `-ca` in the client to import its cert as CA.
- Under normal circumstances, it is recommended to use `-ca` to directly load certificate file. `-cca` is recommended for Android. In Android system it's very inconvenient to transfer and load files.

---

