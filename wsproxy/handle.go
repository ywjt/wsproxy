// Copyright 2023 The WebSocket Proxy Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// @Author: YWJT / ZhiQiang Koo
// @Modify: 2023-03-13  
//

package main


import (
    "gorilla/websocket"
    "crypto/aes256cbc"
    "io"
    "net"
    "net/http"
    "bufio"
    "sync"
    "time"
    "fmt"
    "strings"
)

var (
    pool = make(map[string]p_worker)
    upgrader = websocket.Upgrader{}
    
    max_connections int = 65535
    lock = sync.Mutex{}
    
    codeOK          = 200 //正常握手
    codeDialErr     = 502 //后端服务不可用或没响应
    codeCloseErr    = 503 //后端服务异常断开
    codeDialTimeout = 504 //后端服务连接超时
	
    copyBufPool      sync.Pool
)

type p_worker struct {
	key  string
	format int
	ws *websocket.Conn
	sock net.Conn
}

func setMaxConns(n int) { max_connections = n }


func init() {
    //run websocket
    http.HandleFunc("/", handleWs)
	
    copyBufPool.New = func() interface{} {
        buf := make([]byte, cfgBufferSize)
        return &buf
    }
}


func handleWs(w http.ResponseWriter, r* http.Request) {
    var _t = time.Now()
    var _h = hashCodes([]string{fmt.Sprintf("%s_%s", r.RemoteAddr, _t)})
    
    //==================
    /**
    // 初始化 Upgrader对象
    //
        type Upgrader struct {
             HandshakeTimeout time.Duration //握手超时时间
             ReadBufferSize, WriteBufferSize int //读、写缓冲区大小（默认4096字节）
             WriteBufferPool BufferPool //写缓冲区池
             Subprotocols []string //子协议
             Error func(w http.ResponseWriter, r *http.Request, status int, reason error) //错误函数
             CheckOrigin func(r *http.Request) bool //跨域校验函数
             EnableCompression bool //是否压缩
        }
    **/
    var upgrader = websocket.Upgrader{
        HandshakeTimeout: 5 * time.Second,
        ReadBufferSize:  int(cfgBufferSize),
        WriteBufferSize: int(cfgBufferSize),
        CheckOrigin: func(r *http.Request) bool {
            return true
        },
        EnableCompression: true,
    }
    
    //跨域
    w.Header().Set("Access-Control-Allow-Origin", "*")

    //启用Websocket
    ws, err := upgrader.Upgrade(w, r, nil)
    if _, ok := err.(websocket.HandshakeError); ok {
        return
    } else if err != nil {
		logger.Warningf("webSocket upgrade err, %s", err)
		return
	}

	if len(pool) >= max_connections {
		ws.WriteJSON(map[string]string{
			"error" : "too many connections",
		})
		ws.Close()
		return
	}
	
    //收到加密串进行解码
    var fromValueTrim string
    fromValueTrim = strings.Replace(r.FormValue(cfgFormKey), " ", "+", -1)
    encrypted := strings.TrimSpace(fromValueTrim)
    
    //Token切割取样,某些时候可能会带?号,加上-fsplit可以用于切割
    if cfgFormSplit != "" {
        if strings.Contains(encrypted, cfgLfSplit) {
            encrypted = strings.Split(encrypted, cfgLfSplit)[cfgRfSplit]
        }
    }
    
   // 同时兼容加密与非加密token,也可强制使用加密
    raddr := tokenModel(aesOnly, encrypted)
    if raddr == "__CANTNOT_DECRYPT__" {
        ws.Close()
        return
    }
    
    //处理掉一些加密过程中的特殊字符, 如空格 \r\n
    raddr = strings.TrimSpace(raddr)
    
	var format int
	switch cfgBuffFormat {
        case "text": format = websocket.TextMessage
        case "bin": format = websocket.BinaryMessage
        default: format = websocket.BinaryMessage
	}

	//连接后端TCP
	sock, err := net.DialTimeout(cfgBuffProto, raddr, time.Duration(cfgDialTimeout))
	//timeout
    if ne, ok := err.(net.Error); ok && ne.Timeout() {
        //504 后端服务连接超时
        go log(sock, r, raddr, time.Since(_t), codeDialTimeout, _h).Out()
        ws.Close()
        return
	}
	if err != nil {
        //502 后端服务不可用或没响应
        go log(sock, r, raddr, time.Since(_t), codeDialErr, _h).Out()
        ws.Close()
		return
	}

    //a woker channel
	client := p_worker{_h, format, ws, sock}
    
    //记录一条请求日志集
    go log(sock, r, raddr, time.Since(_t), codeOK, _h).Out()
    //==================

	lock.Lock()
	pool[client.key] = client
	pool[client.key].start()
	lock.Unlock()
}


func aesDecrypt(encrypted string) string{
    _a, err := aes256cbc.DecryptString(cfgSecret, encrypted)
    if err != nil {
        logger.Errorf("Decrypt an error occurred: %s, Encrypt: %s", err, encrypted)
        return "__CANTNOT_DECRYPT__"
    }
    return _a
}

func tokenModel(aes bool, encrypted string) string{
    if aes == true {
        return aesDecrypt(encrypted)
    }
    
    if len(strings.Split(encrypted, ":")) == 2 {
        return encrypted
    }else{
        return aesDecrypt(encrypted)
    }
}


func (p p_worker) start() {
	go p.frontend()
	go p.backend()
}

func (p p_worker) release() {
	p.sock.Close()
	p.ws.Close()

	lock.Lock()
	delete(pool, p.key)
	lock.Unlock()
}

// Socket stream to Websocket channel
func (p *p_worker) backend() {
	reader := bufio.NewReader(p.sock)
	//buf := make([]byte, cfgBufferSize)
	b := copyBufPool.Get().(*[]byte)
        buf := *b
	for {
            // Read from Socket
	    n, err := reader.Read(buf)
	    if err != nil {
                if err == io.EOF {
                    logger.Noticef("[Sock -> Ws] socket read error '%s', User-Id:%s", err, p.key)
                    //500 后端异常断开
                 }
                 break
            }
        
            // Write to Websocket
            err = p.ws.WriteMessage(p.format, buf[:n])
            if err != nil {
                logger.Errorf("[Sock -> Ws] websocket write error: %s, User-Id:%s", err, p.key)
                //400 WS代理同步写异常
                break
            }
	}
        copyBufPool.Put(b)
	p.release()
}

// Websocket stream to Socket channel
func (p *p_worker) frontend() {
	writer := bufio.NewWriter(p.sock)
	for {
		// Read from Websocket
		_, buf, err := p.ws.ReadMessage()
		/**
            // Close codes defined in RFC 6455, section 11.7.
            const (
                CloseNormalClosure           = 1000  //正常关闭
                CloseGoingAway               = 1001  //关闭中
                CloseProtocolError           = 1002  //协议错误
                CloseUnsupportedData         = 1003  //不支持的数据
                CloseNoStatusReceived        = 1005  //无状态接收
                CloseAbnormalClosure         = 1006  //异常关闭
                CloseInvalidFramePayloadData = 1007  //无效的载体数据
                ClosePolicyViolation         = 1008  //违反策略
                CloseMessageTooBig           = 1009  //消息体太大
                CloseMandatoryExtension      = 1010  //强制过期
                CloseInternalServerErr       = 1011  //内部服务器错误
                CloseServiceRestart          = 1012  //服务重启
                CloseTryAgainLater           = 1013  //稍后再试
                CloseTLSHandshake            = 1015  //TLS握手
            )
            **/
            if err != nil {
                //503 后端服务异常断开 (!=1001/1005)
                if websocket.IsUnexpectedCloseError(err,
                                                    websocket.CloseNormalClosure,
                                                    websocket.CloseGoingAway, 
                                                    websocket.CloseNoStatusReceived) {

                    logger.Errorf("[Ws -> Sock] websocket read error: %s, User-Id:%s", err, p.key)
                }else{
                    //客户端主动断开
                    logger.Noticef("%v, User-Id:%s", err, p.key)
                }
                break
            }

		// Write to socket
		n, err := writer.Write(buf)
		if err != nil || n < len(buf) {
            logger.Warningf("[Ws -> Sock] socket write error: %s, User-Id:%s", err, p.key)
            //403 后端数据同步写异常
            break
        }
		writer.Flush()
	}
    
	p.release()
}
