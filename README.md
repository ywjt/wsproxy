# wsproxy
Wsproxy是一个将websocket转成tcp的代理，用了此代理之后，可以直接用原来的tcp服务器，然后客户端用websocket进行通信。

编译
go build wsproxy.go

用法 usage: ./wsproxy -addr 0.0.0.0:1443 -secret test4399

可用参数：./wsproxy -help
