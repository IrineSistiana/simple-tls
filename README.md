# simple-tls

可能是最简单的tls/wss插件。

The `README.md` is also available in English: `README_en.md`

---

在这里下载：[release](https://github.com/IrineSistiana/simple-tls/releases)

## 命令

        client bind addr               server bind addr
               |                             |
    |client|-->|simple-tls client|--TLS1.3-->|simple-tls server|-->|final destination|
                                             |                     |   
                                       client dst addr       server dst addr  

    # 通用参数
    -b string
        [Host:Port] 监听地址
    -d string
        [Host:Port] 目的地地址
    -wss
        使用 Websocket Secure 协议，该选项服务端与客户端需一致(都有或都没有) 
    -path string
        Websocket 的路径

    # 作为客户端运行
    -n string
        服务器证书名
    -cca string
        用于验证服务器身份的PEM格式CA证书的base64编码

    # 作为服务器运行
    -s    
        以服务端模式运行 
    -cert string
        [Path] PEM格式的证书位置
    -key string
        [Path] PEM格式的密钥位置

    # 其他不重要参数
    -gen-cert
        快速生成一个ecc证书
    -cpu int
        允许使用几个CPU
    -fast-open
        启用TCP fast open，仅支持 Linux内核 4.11+
    -t int
        空连接超时时间 (默认 300s)

## 单独使用

    作为服务器: 
    simple-tls -b 0.0.0.0:1080 -d 127.0.0.1:12345 -s -key /path/to/your/key -cert /path/to/your/cert
    作为客户端:
    simple-tls -b 127.0.0.1:1080 -d your_server_ip:1080 -n your.server.certificates.dnsname

## 作为SIP003插件使用

遵守shadowsocks [SIP003](https://shadowsocks.org/en/spec/Plugin.html)插件协议。接受的键值对[同上](#命令)。shadowsocks会自动设定`-d` `-b`参数，无需手动设定。

以[shadowsocks-libev](https://github.com/shadowsocks/shadowsocks-libev)为例：

    # TLS
    ss-server -c config.json --plugin simple-tls --plugin-opts "s;key=/path/to/your/key;cert=/path/to/your/cert"
    ss-local -c config.json --plugin simple-tls --plugin-opts "n=your.server.certificates.dnsname"

    # WSS
    ss-server -c config.json --plugin simple-tls --plugin-opts "s;wss;key=/path/to/your/key;cert=/path/to/your/cert"
    ss-local -c config.json --plugin simple-tls --plugin-opts "wss;n=your.server.certificates.dnsname"

## Android

是[shadowsocks-android](https://github.com/shadowsocks/shadowsocks-android)的插件。需先安装shadowsocks-android。

## Tips

`-gen-cert` 可以快速的生成一个ECC证书，并打印出base64编码后的cert的用于客户端用`-cca`导入。证书DNSName取自`-n`参数或随机生成。key和cert文件会放在`-key`，`-cert`指定的位置或当前目录`./`。比如：

    simple-tls -gen-cert -n example.com

---

仅供个人娱乐学习交流使用
