module github.com/IrineSistiana/simple-tls

go 1.14

require (
	github.com/IrineSistiana/ctunnel v0.0.0-20210409113947-9756ebc29fdb
	github.com/kr/pretty v0.1.0 // indirect
	github.com/stretchr/testify v1.7.0 // indirect
	github.com/xtaci/smux v1.5.16
	golang.org/x/net v0.0.0-20211112202133-69e39bad7dc2
	golang.org/x/sys v0.0.0-20211003122950-b1ebd4e1001c
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	nhooyr.io/websocket v1.8.7
)

replace github.com/xtaci/smux v1.5.16 => github.com/IrineSistiana/smux v1.5.17
