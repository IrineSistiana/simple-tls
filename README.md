# simple-tls

简单易用的 TCP 连接转发器。可为原始数据流加一层 TLS。支持通过 gRPC 传输。

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
  -grpc
      使用 gRPC 协议。客户端和服务端需一致。
  -grpc-path string
      (可选) gRPC 服务路径。客户端和服务端需一致。

# 客户端参数
# e.g. simple-tls -b 127.0.0.1:1080 -d your_server_ip:1080 -n your.server.name

  -n string
      服务器证书名。用于验证服务端的证书的合法性。也用作 SNI。
  -no-verify
      客户端将不会验证服务端的证书的合法性。(证书链验证)
  -ca string
      用于验证服务端的证书的 CA 证书文件。(默认使用系统证书池)
  -cert-hash string
      服务器证书的 hash。(服务端证书锁定)
      tips: 使用 -hash-cert 命令可以生成证书的 hash

# 服务端参数
# e.g. simple-tls -b :1080 -d 127.0.0.1:12345 -s -key /path/to/your/key -cert /path/to/your/cert
# 证书格式必须是 PEM (base64) 。
# -cert 和 -key 可以同时留空，会在内存中生成一个临时证书。证书的域名默认随机，但也可以取自 `-n` 参数。
# e.g. simple-tls -b :1080 -d 127.0.0.1:12345 -s -n my.test.domain

  -s    
      (必需) 以服务端运行。
  -cert string
      证书路径。
  -key string
      密钥路径。

# 其他通用参数

  -t int
      连接空闲超时，单位秒 (默认300)。
  -outbound-buf int
      设置出站 tcp rw socket buf。
  -inbound-buf    
      设置入站 tcp rw socket buf。

# 命令

  -gen-cert
      生成一个密钥长度为 256 的 ECC 证书到当前目录。
      证书的 dns name 可以用 `-n` 设定。默认是随机字符串。
      可以用 `-template` 指定模板证书。除密钥等关键参数外，其他参数都会从模板证书复制。
      可以用 `-cert` 和 `-key` 指定证书输出位置。(默认当前目录且文件名是证书的 dns name)
      e.g. simple-tls -gen-cert -n my.domain
      会生成证书 my.domain.cert 和密钥 my.domain.key 两个文件到当前目录。
  -hash-cert
      显示证书的 hash 值。(用于客户端的 -cert-hash)
      e.g. simple-tls -hash-cert ./my.cert
  -v
      显示目前程序版本
```

## 服务端无合法证书时如何快速使用 

服务端使用临时证书，客户端不做任何验证。下层连接有安全措施时可以使用该方案。

```shell
# 服务端的 -cert 和 -key 同时留空，会在内存生成一个临时证书。
simple-tls -b :1080 -d 127.0.0.1:12345 -s -n my.cert.domain
# 客户端禁用证书链验证。
simple-tls -b :1080 -d your.server.address:1080 -n my.cert.domain -no-verify
```

服务端使用固定证书，客户端使用 hash 验证服务端证书 (证书锁定)。

```shell
# 服务端生成一个证书。
simple-tls -gen-cert -n my.cert.domain
# 然后显示证书的 hash。e.g. 8910fe28d2fb40398a...
simple-tls -hash-cert ./my.cert.domain.cert
# 使用这个证书启动服务端
simple-tls -b :1080 -d 127.0.0.1:12345 -s -key ./my.cert.domain.key -cert ./my.cert.domain.cert
# 客户端禁用证书链验证但启用证书 hash 验证。
simple-tls -b :1080 -d your.server.address:1080 -n my.cert.domain -no-verify -cert-hash 8910fe28d2fb40398a...
```

## 作为 SIP003 插件使用

支持 shadowsocks 的 [SIP003](https://shadowsocks.org/en/wiki/Plugin.html) 插件协议。shadowsocks 主程序会自动设定监听地址 `-b` 和目的地地址 `-d`。

以 [shadowsocks-rust](https://github.com/shadowsocks/shadowsocks-rust) 为例:

```shell
ssserver -c config.json --plugin simple-tls --plugin-opts "s;key=/path/to/your/key;cert=/path/to/your/cert"
sslocal -c config.json --plugin simple-tls --plugin-opts "n=your.server.certificates.dnsname"
```

### Android SIP003 插件

simple-tls-android 是 [shadowsocks-android](https://github.com/shadowsocks/shadowsocks-android) 的带 GUI 的插件。目前随 simple-tls 一起发布。可从 release 界面下载全平台通用的 apk。

simple-tls-android 的源代码在 [这里](https://github.com/IrineSistiana/simple-tls-android) 。

### Beta 版本

simple-tls 目前不保证版本之间的兼容性。