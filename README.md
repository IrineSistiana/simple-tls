# simple-tls

可能是最简单的tls插件。

---

特点：

* 简单：除TLS加密不提供格外的功能 ~~这叫特点?~~
* 安全：强制使用TLS1.3。
* 支持shadowsocks插件。
* 支持Android。

## 命令

    |client|-->|simple-tls client|--TLS1.3-->|simple-tls server|-->|destination|

    # 服务端与客户端都需要(插件模式除外)
    -b string
        [Host:Port] 监听地址 (必需，插件模式除外)
    -d string
        [Host:Port] 目的地地址 (必需，插件模式除外)

    # 客户端模式
    -cca string
        客户端用于验证服务器的base64编码的PEM格式CA证书。
        如果服务端证书是合法证书的话一般不需要此参数，
        simple-tls会使用系统的证书池去验证证书。
    -n string
        服务端名称，用于证书host name验证 (必需)

    # 服务端模式
    -s    
        以服务端模式运行，不加此参数就变客户端了 (必需)
    -cert string
        [Path] PEM格式的证书 (必需)
    -key string
        [Path] PEM格式的密钥 (必需)

    # 其他
    -gen-cert
        [This is a helper function]: generate a certificate, store it's key to [-key] and cert to [-cert], print cert in base64 format
    -cpu int
        the maximum number of CPUs that can be executing simultaneously
    -fast-open
        enable tfo, only available on linux 4.11+
    -t int
        timeout after sec (default 300)

## SIP003

支援shadowsocks [SIP003](https://shadowsocks.org/en/spec/Plugin.html)插件协议。接受的键值对[同上](#命令)。

以SIP003插件模式运行时，`b`,`d`参数由shadowsocks自动设定，无需再次在`plugin-opts`中设定。

以[shadowsocks-libev](https://github.com/shadowsocks/shadowsocks-libev)为例：

    ss-server -c config.json --plugin simple-tls --plugin-opts "s;key=/path/to/your/key;cert=/path/to/your/cert"
    ss-local -c config.json --plugin simple-tls --plugin-opts "n=your.server.certificates.dnsname"

## Android

simple-tls-android是[shadowsocks-android](https://github.com/shadowsocks/shadowsocks-android)的插件.

支援Android 7以上系统。

## Tips

`-gen-cert` 可以快速的生成一个ECC证书，并打印出base64编码后的cert的用于客户端用`-cca`导入。证书DNSName取自`-n`参数或随机生成。key和cert文件会放在`-key`，`-cert`指定的位置或当前目录`./`。

条件允许的话还是建议从[Let's Encrypt](https://letsencrypt.org/)整一个合法的证书。

高级用户建议用nginx，又小又快老牌软件稳定可靠功能全。参考nginx官方文档：[Securing TCP Traffic to Upstream Servers](https://docs.nginx.com/nginx/admin-guide/security-controls/securing-tcp-traffic-upstream/)。simple-tls只是配置和使用简单点。

tls 1.3的加密强度足够。下层的加密强度可降低或不加密。

仅供个人娱乐学习交流使用
