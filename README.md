# simple-tls

[中文](README.md) [English](README_en.md)

---

- [simple-tls](#simple-tls)
  - [参数](#参数)
  - [SIP003模式](#sip003模式)
  - [Android](#android)

## 参数

```text
      客户端监听地址               服务端监听地址
           |                            |
|客户端|-->|simple-tls 客户端|--TLS1.3-->|simple-tls 服务端|-->|最终目的地|
                                        |                     |   
                                   客户端目的地地址     服务端目的地地址  

# 通用参数
  -b string
      [Host:Port] 监听地址。
  -d string
      [Host:Port] 目的地地址。
  -auth string
      身份验证密码。(可选。客户端和服务端需一致。仅用于过滤扫描流量。)

# 客户端参数
# e.g. simple-tls -b 127.0.0.1:1080 -d your_server_ip:1080 -n your.server.name

  -mux int
      单条 TCP 连接内最大复用的连接数。(默认 0 禁用 mux)
  -n string
      服务器证书名。
  -no-verify
      客户端将不会验证服务端的证书。
  -ca string
      加载 PEM CA 证书文件。
      e.g. -ca ./path/to/my.ca.cert
  -cca string
      从字符串加载被 base64 编码 (e.g. base64 -w 0 ./my.cert) 的 PEM CA 证书。
      e.g. -cca VkRJW...4eGdFbz0K==

# 服务端参数
# e.g. simple-tls -b 0.0.0.0:1080 -d 127.0.0.1:12345 -s -key /path/to/your/key -cert /path/to/your/cert
# -cert 和 -key 可以同时留空，会生成一个临时证书。证书的 Subject Alternate Name 取自 `-n` 参数。
# e.g. simple-tls -b 0.0.0.0:1080 -d 127.0.0.1:12345 -s -n my.test.domain

  -s    
      以服务端运行。
  -cert string
      PEM 证书路径。
  -key string
      PEM 密钥路径。

# 其他参数

  -cpu int
      最多使用的cpu数。
  -fast-open
      启用TCP快速开启，仅支持Linux内核4.11+。
  -t int
      空闲超时，以秒记 (默认300)。

# 命令

  -gen-cert
      快速生成一个 Subject Alternate Name 为 `-n` 的 ECC 证书
      e.g. simple-tls -gen-cert -n my.test.domain
  -v
      显示目前程序版本
```

## SIP003模式

支持 [SIP003](https://shadowsocks.org/en/wiki/Plugin.html) 插件协议。 以 [shadowsocks-libev](https://github.com/shadowsocks/shadowsocks-libev) 为例:

```shell
ss-server -c config.json --plugin simple-tls --plugin-opts "s;key=/path/to/your/key;cert=/path/to/your/cert"
ss-local -c config.json --plugin simple-tls --plugin-opts "n=your.server.certificates.dnsname"
```

## Android

simple-tls-android 是 [shadowsocks-android](https://github.com/shadowsocks/shadowsocks-android) 的GUI插件，需要先下载 shadowsocks-android。simple-tls-android 同样是开源软件，源代码在 [这里](https://github.com/IrineSistiana/simple-tls-android) 。

<details><summary><code>屏幕截图</code></summary>

<br>

![截屏](/assets/simple-tls-android-screenshot.jpg)

</details>

---
