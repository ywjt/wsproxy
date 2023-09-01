# wsproxy

Wsproxy是一个将websocket转成tcp的代理，用了此代理之后，可以直接用原来的tcp服务器，然后客户端用websocket进行通信。
```
WSproxy v2.3.1 beta
- 2023-06-30 [优化]增加 X-Forwarded-For 头部，用于后端获取真实IP
- 2023-03-24 [新增]支持proxy protocol协议，以便后端服务器获取客户端真实ip
- 2023-03-17 [新增]支持 ws 后端代理协议
- 2023-03-17 优化BufferSize缓冲区，发现使用大缓冲区读写时会导致协议粘包，建议业务协议里加入包头识别
- 2023-03-13 [优化]内存分配，减轻GC压力
- 2023-03-13 [新增]支持 text/binary 转发流格式
- 2023-03-13 [新增]支持 tcp/udp 后端代理协议
- 2023-03-13 [新增]支持 max_conns 最大代理连接数
- 高并发性能，资源消耗低
- 网关支持前端ws、wss服务协议
- 使用aes加密算法，后端ip地址不直接对外
- 没有多余的配置，可作为全局代理网关
```
### 性能测试
<img src='https://github.com/ywjt/wsproxy/blob/main/doc/wsproxy_performance_testing.png'> 
PS: CPU E5-2699 v3 2.30GHz、8核、16G (仅开4个核)。 保守支持 1W并发连接，20W pps。


### 编译:

```bash
#进入主目录
export PATH=$PATH:/usr/local/go/bin:`pwd`
export GOPATH=`pwd`
go env -w GO111MODULE=auto
go get -u golang.org/x/sys/unix

cd wsproxy
go build .  #可能还需要安装必须的依赖
```

### 用法:
```bash
usage: ./wsproxy -addr 0.0.0.0:1443 -secret test1234
```

### 制作Docker镜像:
将当前目录下编译好的二进制文件，复制到 bin文件夹，并编写Dockerfile 进行打包。
```bash
mv wsproxy ../bin/

cat >Dockerfile <<EOF
FROM busybox
WORKDIR /
COPY wsproxy /

EXPOSE 1443
LABEL org.opencontainers.image.authors="SunshineKoo"
LABEL org.opencontainers.image.version="2.3.1-beta"

ENTRYPOINT ["./wsproxy"]
CMD ["-h"]
EOF

docker build -t wsproxy:2.3.1 .
```

**启动容器：**
```bash
docker run --name wsproxy -d -p 1443:1443 wsproxy:2.3.1 -secret test1234
docker ps -a|grep wsproxy
```


### 可用参数：

```help
[root@~ ]# ./wsproxy -h

Usage of ./wsproxy:
  -addr string
        Network address for gateway (default "0.0.0.0:1443")
  -aes_only
        Run WSproxy on encryption mode for AES
  -buffer uint
        Buffer size for ReadBuffer()/WriteBuffer() (default 1024)
  -frkey string
        Key name for URL request. like '/?token=xeR7LpmprJS8U...'
        (Exp: -frkey token or -frkey token123) Fmt: ^[a-z]+[0-9]*  (default "token")
  -fsplit string
        Split token from formValue, like '?t=xeR7LpmprJS8U...?v=4693225'
        (Exp: -fsplit "?v=",0 )  Res: 'xeR7LpmprJS8U...' 
  -max_conns uint
        Max connections to slots available. (default 65536)
  -secret string
        The passphrase used to decrypt target server address
  -ssl_cert string
        SSL certificate file (default "./cert.pem")
  -ssl_key string
        SSL key file (if separate from cert) (default "./key.pem")
  -ssl_only
        Run WSproxy for TLS version
  -stream string
        Buffer stream format for (text, bin). Only TCP/UDP backend.
        (Exp: -stream bin or -stream text ) (default "bin")
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
**加密方式：**
```
ws://your-domain:1443/?token=U2FsdGVkX1+G76LHp6mvNpyMSqR1WoGGTcSLIyD+/7A=
ws://your-domain:1443/ws?token=U2FsdGVkX1+G76LHp6mvNpyMSqR1WoGGTcSLIyD+/7A=
```

支持开启TLS：
```
wss://your-domain:1443/?token=U2FsdGVkX1+G76LHp6mvNpyMSqR1WoGGTcSLIyD+/7A=
wss://your-domain:1443/ws?token=U2FsdGVkX1+G76LHp6mvNpyMSqR1WoGGTcSLIyD+/7A=
```

**非加密方式：**
```
ws://your-domain:1443/?token=127.0.0.1:80
wss://your-domain:1443/?token=127.0.0.1:80
ws://your-domain:1443/ws?token=127.0.0.1:80
wss://your-domain:1443/ws?token=127.0.0.1:80
```

**按代理协议请求**

网关能复用代理端口进行不同后端协议的转换。

| 请求URL | 后端协议 | 说明 |
| :---- | :----: | :---- |
| /?token= | TCP | 从网关WS/WSS --> 后端TCP (必须是tcp协议) |
| /udp?token= | UDP | 从网关WS/WSS --> 后端UDP (未测试) |
| /ws?token= | WS | 从网关WS/WSS --> 后端WS (必须是ws协议) |

*注意必须匹配好对应的后端协议，否则代理不成功。

