# wsproxy

Wsproxy是一个将websocket转成tcp的代理，用了此代理之后，可以直接用原来的tcp服务器，然后客户端用websocket进行通信。
```
- 高并发性能，资源消耗低
- 支持ws、wss服务协议
- 使用aes加密算法，后端ip地址不直接对外
- 没有多余的配置，可作为全局代理网关
```
### 性能测试
<img src='https://github.com/ywjt/wsproxy/blob/main/doc/wsproxy_performance_testing.png'> 
PS: CPU E5-2699 v3 2.30GHz、8核、16G (仅开4个核)。 支持 1W并发连接，20W pps。


### 编译:

```bash

export PATH=$PATH:/usr/local/go/bin:`pwd`
export GOPATH=`pwd`

go build .
```

### 用法:
```bash
usage: ./wsproxy -addr 0.0.0.0:1443 -secret test1234
```

### 可用参数：

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

### 加密：

客户端发送到网关的目标服务器地址使用AES256-CBC加密并进行base64编码，密文以换行符结尾。

示例：
```
U2FsdGVkX19KIJ9OQJKT/yHGMrS+5SsBAAjetomptQ0=
```
进行加密目的是为了隐藏后端服务IP地址和端口，保证较高的安全性。而网关是可以作为全局代理，所有业务均可无缝使用。

使用AES算法加密文本格式的后端地址，生成base64编码的密文。可以在线生成：http://tool.oschina.net/encrypt

也可以使用openssl命令生成，如：
```bash
echo -n "127.0.0.1:8088" | openssl enc -e -aes-256-cbc -a -salt -k "test1234"
```

举例，当后端地址为127.0.0.1:8088并且Secret为 testr1234 时，密文结果应类似：
```
U2FsdGVkX1+G76LHp6mvNpyMSqR1WoGGTcSLIyD+/7A=
```

注：上述方式都会使用随机Salt，这也是建议的方式。其结果是每次加密得出的密文结果并不一样，但并不会影响解密。


### 请求方法

```
ws://your-domain:1443/?token=U2FsdGVkX1+G76LHp6mvNpyMSqR1WoGGTcSLIyD+/7A=
```

支持开启TLS：
```
wss://your-domain:1443/?token=U2FsdGVkX1+G76LHp6mvNpyMSqR1WoGGTcSLIyD+/7A=
```
