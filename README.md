# wsproxy
Wsproxy是一个将websocket转成tcp的代理，用了此代理之后，可以直接用原来的tcp服务器，然后客户端用websocket进行通信。

编译:

```bash
/bin/cp -rf {crypto,gologger,gorilla} /usr/local/go/src/
go build wsproxy.go
```

用法:
```bash
usage: ./wsproxy -addr 0.0.0.0:1443 -secret test1234
```

可用参数：

```help
[root@~ ]# ./wsproxy -h

Usage of ./wsproxy:
  -addr string
        Network address for gateway (default "0.0.0.0:1443")
  -buffer uint
        Buffer size for ReadBuffer()/WriteBuffer() (default 65536)
  -frkey string
        Key name for URL request. like '/?token=xeR7LpmprJS8U...'
        (Exp: -frkey token or -frkey token123) Fmt: ^[a-z]+[0-9]*  (default "token")
  -fsplit string
        Split token from formValue, like '?t=xeR7LpmprJS8U...?v=4693225'
        (Exp: -fsplit "?v=",0 )  Res: 'xeR7LpmprJS8U...' 
  -secret string
        The passphrase used to decrypt target server address
  -ssl_cert string
        SSL certificate file (default "./cert.pem")
  -ssl_key string
        SSL key file (if separate from cert) (default "./key.pem")
  -ssl_only
        Run WSproxy for TLS version
  -timeout uint
        Timeout seconds when dial to targer server (default 3)
  -version
        Print WSproxy version
```
