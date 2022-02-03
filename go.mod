module github.com/IrineSistiana/simple-tls

go 1.17

require (
	github.com/IrineSistiana/ctunnel v0.0.0-20210409113947-9756ebc29fdb
	github.com/xtaci/smux v1.5.16
	golang.org/x/sys v0.0.0-20220128215802-99c3d69c2c27
	nhooyr.io/websocket v1.8.7
)

require (
	github.com/klauspost/compress v1.14.2 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
)

replace github.com/xtaci/smux v1.5.16 => github.com/IrineSistiana/smux v1.5.16-mod
