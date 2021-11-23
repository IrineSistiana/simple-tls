# simple-tls

一个简单易用的 shadowsocks SIP003 插件。可为原始数据流加一层 TLS。还支持 Websocket 和 mux (连接复用)。

---

## 参数

```text
      客户端监听地址               服务端监听地址
           |                            |
|客户端|-->|simple-tls 客户端|--TLS1.3-->|simple-tls 服务端|-->|最终目的地|
                                        |                     |   
                                   客户端目的地地址     服务端目的地地址  

# 通用参数
  -b string
      [Host:Port] (必需) 监听地址。
  -d string
      [Host:Port] (必需) 目的地地址。
  -auth string
      身份验证密码。客户端和服务端需一致。用于过滤扫描流量。
  -ws
      使用 Websocket 协议。客户端和服务端需一致。
  -ws-path string
      Websocket 的 url path。客户端和服务端需一致。

# 客户端参数
# e.g. simple-tls -b 127.0.0.1:1080 -d your_server_ip:1080 -n your.server.name

  -mux int
      单条 TCP 连接内最大复用的连接数。(默认 0, 禁用 mux)
  -n string
      服务器证书名。
  -no-verify
      客户端将不会验证服务端的证书的合法性。(证书链验证)
  -ca string
      加载 CA 证书文件。
      e.g. -ca ./path/to/my.ca.cert
  -cert-hash string
      检查服务器证书的 hash。(文件验证)

# 服务端参数
# e.g. simple-tls -b 0.0.0.0:1080 -d 127.0.0.1:12345 -s -key /path/to/your/key -cert /path/to/your/cert
# -cert 和 -key 可以同时留空，会生成一个临时证书。证书的 Subject Alternate Name 取自 `-n` 参数。
# e.g. simple-tls -b 0.0.0.0:1080 -d 127.0.0.1:12345 -s -n my.test.domain

  -s    
      (必需) 以服务端运行。
  -cert string
      证书路径。
  -key string
      密钥路径。
  -no-tls
      禁用 TLS

# 其他参数

  -fast-open
      启用 tcp fast open，仅支持 Linux 4.11+ 内核。
  -t int
      空闲超时，以秒记 (默认300)。

# 命令

  -gen-cert
      快速生成一个 Subject Alternate Name 为 `-n` 的 ECC 证书。
      e.g. simple-tls -gen-cert -n my.test.domain
  -hash-cert
      计算证书的 hash 值。(用于客户端的 -cert-hash)
      e.g. simple-tls -hash-cert ./my.cert
  -v
      显示目前程序版本
```

## SIP003 模式

支持 shadowsocks 的 [SIP003](https://shadowsocks.org/en/wiki/Plugin.html) 插件协议。 以 [shadowsocks-libev](https://github.com/shadowsocks/shadowsocks-libev) 为例:

```shell
ss-server -c config.json --plugin simple-tls --plugin-opts "s;key=/path/to/your/key;cert=/path/to/your/cert"
ss-local -c config.json --plugin simple-tls --plugin-opts "n=your.server.certificates.dnsname"
```

## Android

simple-tls-android 是 [shadowsocks-android](https://github.com/shadowsocks/shadowsocks-android) 的带 GUI 的插件。

目前随 simple-tls 一起发布。可从 release 界面下载。

simple-tls-android 的源代码在 [这里](https://github.com/IrineSistiana/simple-tls-android) 。

---
