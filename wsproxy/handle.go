// Copyright 2023 The WebSocket Proxy Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// @Author: YWJT / ZhiQiang Koo
// @Modify: 2023-03-24
//

package main


import (
    "gorilla/websocket"
    "crypto/aes256cbc"
    "proxyproto"
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
    wc *websocket.Conn
	sock net.Conn
}

func setMaxConns(n int) { max_connections = n }

func init() {
    //run websocket handle
    http.HandleFunc("/", TCP)
    http.HandleFunc("/udp", UDP)
    http.HandleFunc("/ws", WSS)
    
    copyBufPool.New = func() interface{}{
        buf := make([]byte, cfgBufferSize)
        return &buf
    }
}


//handle functions
func TCP(w http.ResponseWriter, r *http.Request){ 
    if r.URL.Path == "/" {
        handles(w, r, "tcp") 
    }
    http.StatusText(400)
}
func UDP(w http.ResponseWriter, r *http.Request){ 
    if r.URL.Path == "/udp" {
        handles(w, r, "udp") 
    }
    http.StatusText(400)
}
func WSS(w http.ResponseWriter, r *http.Request){ 
    if r.URL.Path == "/ws" {
        handles(w, r, "wss") 
    }
    http.StatusText(400)
}


// ************************************************************
// Initial Upgrader
//
// type Upgrader struct {
//     HandshakeTimeout time.Duration //握手超时时间
//     ReadBufferSize, WriteBufferSize int //读、写缓冲区大小（默认4096字节）
//     WriteBufferPool BufferPool //写缓冲区池
//     Subprotocols []string //子协议
//     Error func(w http.ResponseWriter, r *http.Request, status int, reason error) //错误函数
//     CheckOrigin func(r *http.Request) bool //跨域校验函数
//     EnableCompression bool //是否压缩
// }
// ************************************************************
func handleShake(w http.ResponseWriter, r *http.Request) (ws *websocket.Conn, raddr string){
    
    var upgrader = websocket.Upgrader{
        HandshakeTimeout: time.Duration(cfgDialTimeout),
        ReadBufferSize:  int(cfgBufferSize),
        WriteBufferSize: int(cfgBufferSize),
        CheckOrigin: func(r *http.Request) bool {
            return true
        },
        EnableCompression: true,
    }
    
    //make header
    //w.Header().Set("Access-Control-Allow-Origin", "*")
    //w.Header().Set("Access-Control-Allow-Headers", "X-Requested-With")
    //w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
    x_real_ip := strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]
    x_remote_addr,_,_ := net.SplitHostPort(r.RemoteAddr)
    x_real_ip = If(x_real_ip == "", x_remote_addr, x_real_ip).(string)
    w.Header().Set("X-Real-IP", x_real_ip)
    
    ws, err := upgrader.Upgrade(w, r, w.Header())
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
    
   //同时兼容加密与非加密token,也可强制使用加密
    _raddr := tokenModel(aesOnly, encrypted)
    if _raddr == "__CANTNOT_DECRYPT__" {
        return
    }
    
    //处理掉一些加密过程中的特殊字符, 如空格 \r\n
    _raddr = strings.TrimSpace(_raddr)
    
    return ws, _raddr
}

func handles(w http.ResponseWriter, r *http.Request, pt string) {
    var _t = time.Now()
    var _h = hashCodes([]string{fmt.Sprintf("%s_%s", r.RemoteAddr, _t)})
    
    ws, raddr := handleShake(w, r)
    if ws == nil {
        return
    } else if raddr == "" {
        ws.Close()
        return
    }
    
	var format int
	switch cfgBuffFormat {
        case "text": format = websocket.TextMessage
        case "bin": format = websocket.BinaryMessage
        default: format = websocket.BinaryMessage
	}

    var client = p_worker{}
    switch pt {
        case "wss":
            //connect WS/WSS client
            wc, _, err := websocket.DefaultDialer.Dial("ws://"+raddr+"/", w.Header())
            if err != nil {
                //502 bad gateway
                go log(nil, wc, r, raddr, time.Since(_t), codeDialErr, _h).Out()
                ws.Close()
                return
            }
        
            //add a worker
            client = p_worker{_h, format, ws, wc, nil}
            //record a log
            go log(nil, wc, r, raddr, time.Since(_t), codeOK, _h).Out()
        
        default:
            //connect TCP/UDP client
            sock, err := net.DialTimeout(pt, raddr, time.Duration(cfgDialTimeout))
            if ne, ok := err.(net.Error); ok && ne.Timeout() {
                //504 gateway timeout
                go log(sock, nil, r, raddr, time.Since(_t), codeDialTimeout, _h).Out()
                ws.Close()
                return
            }
            if err != nil {
                //502 bad gateway
                go log(sock, nil, r, raddr, time.Since(_t), codeDialErr, _h).Out()
                ws.Close()
                return
            }
        
        
            /*********************************************************
            // 构造一个代理协议头部
            // 如果前端还有代理服务，应在前端同时配置代理协议
            // 目前腾讯云/阿里云均支持 proxy-protocol
            //   <nginx stream> :
            //     listen 80 proxy_protocol;
            //     listen 443 ssl proxy_protocol;
            //
            //   <haproxy tcp> :
            //     server smtp 127.0.0.1:2319 send-proxy    #IPV4 V1
            //     server smtp 127.0.0.1:2319 send-proxy-v2 #IPV4 V2
            // 
            // X-Forwarded-For 经过多层转发后，每被转发一次都会依次记录请求源地址
            // 只需要读取第一个字端，即是客户端的真实IP
            //
            // PS: 直接请求网关时 X-Forwarded-For没有值，使用RemoteAddr函数，
            // 即可以获得真实IP。当 X-Forwarded-For 有值时，即用 X-Forwarded-For
            // 替换 RemoteAddr的值。
            **********************************************************/
            if PProto==true {
                x_real_ip := strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]
                x_localaddr := r.RemoteAddr
                if x_real_ip != "" {
                    x_localaddr = strings.Replace(x_localaddr, "127.0.0.1", x_real_ip, -1)
                }
                // 连接TCP后端成功后，发送第一条信息为 proxy-protocol报文
                send_proxyproto(sock, x_localaddr, raddr)
            }
        
            //add a worker
            client = p_worker{_h, format, ws, nil, sock}
            //record a log
            go log(sock, nil, r, raddr, time.Since(_t), codeOK, _h).Out()
    }

	lock.Lock()
	pool[client.key] = client
	pool[client.key].start(pt)
	lock.Unlock()
}


func send_proxyproto(c net.Conn, laddr, raddr string) bool{
    l_addr,_ := net.ResolveTCPAddr("tcp", laddr)
    r_addr,_ := net.ResolveTCPAddr("tcp", raddr)
    
    _header := &proxyproto.Header{
                Version:            1,
                Command:            proxyproto.PROXY,
                TransportProtocol:  proxyproto.TCPv4,
                SourceAddr: l_addr,
                DestinationAddr: r_addr,
    }

    _, err := _header.WriteTo(c)
    if err != nil {
        logger.Errorf("Error: %s", err.Error())
        return false
    }
    return true
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

func (p p_worker) start(typ string) {
    if typ == "tcp" || typ == "udp" {
        go p.frontend()
        go p.backend()
        
    } else if typ == "wss" {
        go p.upstream()
        go p.downstream()
    }
}

func (p p_worker) release_tup() {
    p.ws.Close()
    p.sock.Close()
	lock.Lock()
	delete(pool, p.key)
	lock.Unlock()
}

func (p p_worker) release_wsp() {
    p.ws.Close()
    p.wc.Close()
	lock.Lock()
	delete(pool, p.key)
	lock.Unlock()
}

// Websocket to Socket
// ************************************************************
// Close codes defined in RFC 6455, section 11.7.
//    const (
//        CloseNormalClosure           = 1000  //正常关闭
//        CloseGoingAway               = 1001  //关闭中
//        CloseProtocolError           = 1002  //协议错误
//        CloseUnsupportedData         = 1003  //不支持的数据
//        CloseNoStatusReceived        = 1005  //无状态接收
//        CloseAbnormalClosure         = 1006  //异常关闭
//        CloseInvalidFramePayloadData = 1007  //无效的载体数据
//        ClosePolicyViolation         = 1008  //违反策略
//        CloseMessageTooBig           = 1009  //消息体太大
//        CloseMandatoryExtension      = 1010  //强制过期
//        CloseInternalServerErr       = 1011  //内部服务器错误
//        CloseServiceRestart          = 1012  //服务重启
//        CloseTryAgainLater           = 1013  //稍后再试
//        CloseTLSHandshake            = 1015  //TLS握手
//    )
//***********************************************************/
func (p *p_worker) frontend() {
	writer := bufio.NewWriter(p.sock)
	for {
		// Read from Websocket
		_, buf, err := p.ws.ReadMessage()
        if err != nil {
            //normal close (!=1000/1001/1005)
            if websocket.IsUnexpectedCloseError(err,
                                                websocket.CloseNormalClosure,
                                                websocket.CloseGoingAway, 
                                                websocket.CloseNoStatusReceived) {

                logger.Errorf("[Ws -> Sock] websocket read error: %s, User-Id:%s", err, p.key)
            }else{
                //err close
                logger.Noticef("%v, User-Id:%s", err, p.key)
            }
            break
        }

		// Write to socket
		n, err := writer.Write(buf)
		if err != nil || n < len(buf) {
            logger.Warningf("[Ws -> Sock] socket write error: %s, User-Id:%s", err, p.key)
            break
        }
		writer.Flush()
	}
	p.release_tup()
}


// Socket to Websocket
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
                 }
                 break
            }
        
            // Write to Websocket
            err = p.ws.WriteMessage(p.format, buf[:n])
            if err != nil {
                logger.Errorf("[Sock -> Ws] websocket write error: %s, User-Id:%s", err, p.key)
                break
            }
	}
    copyBufPool.Put(b)
	p.release_tup()
}


// WebsocketUP to websocketDOWN
func (p *p_worker) upstream() {
	for {
        // Read
		_typ, buf, err := p.ws.ReadMessage()
        if err != nil {
            //normal close (!=1000/1001/1005)
            if websocket.IsUnexpectedCloseError(err,
                                                websocket.CloseNormalClosure,
                                                websocket.CloseGoingAway, 
                                                websocket.CloseNoStatusReceived) {

                logger.Noticef("[Ws -> Wc] websocket read error: %v, User-Id:%s", err, p.key)
            }else{
                //err close
                logger.Noticef("%v, User-Id:%s", err, p.key)
            }
            break
        }
        // Write
        err = p.wc.WriteMessage(_typ, buf)
        if err != nil {
            //logger.Errorf("[Ws -> Wc] websocket write error: %s, User-Id:%s", err, p.key)
            break
        }

	}
	p.release_wsp()
}
    

// WebsocketDOWN to websocketUP
func (p *p_worker) downstream() {
	for {
        // Read
		_typ, buf, err := p.wc.ReadMessage()
        if err != nil {
            //logger.Noticef("[Wc -> Ws]websocket read error: %v, User-Id:%s", err, p.key)
            break
        }
        // Write
        err = p.ws.WriteMessage(_typ, buf)
        if err != nil {
            logger.Errorf("[Wc -> Ws] websocket write error: %s, User-Id:%s", err, p.key)
            break
        }

	}
	p.release_wsp()
}
