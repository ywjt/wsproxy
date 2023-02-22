// Copyright 2023 The WebSocket Proxy Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// @Author: YWJT / ZhiQiang Koo
// @Modify: 2023-02-21   
//

package main

import (
    "gorilla/websocket"
    "crypto/aes256cbc"
    "gologger"
    "fmt"
    "net"
    "net/http"
    "os"
    "os/signal"
    "strings"
    "time"
    "flag"
    "syscall"
    "io"
    "io/ioutil"
    "strconv"
    "hash/crc32"
    "bytes"
    "regexp"
)


var (
    logh       LogHandle
    tlsh       PemTLS
    num        int
    new_addr   string
    cfgSecret  string
    cfgLfSplit string
    cfgRfSplit int
    
    cfgGatewayAddr = "0.0.0.0:1443"
    cfgDialTimeout = uint(3)
    cfgBufferSize  = uint(64 * 1024)
    cfgFormSplit   = ""
    cfgFormKey     = "token"
    
    ws     = websocket.Upgrader{}
    logger = gologger.NewLogger()
    
    __VERSION__ = "1.0.0 beta"
    __SSL_TLS__ = "No support (no cert file)"
    appVersion  = true
    
    cfgCertFile  = "./cert.pem"
    cfgKeyFile   = "./key.pem"
    sslOnly      = true
    
    codeOK          = 200 //正常握手
    codeDialErr     = 502 //后端服务不可用或没响应
    codeCloseErr    = 503 //后端服务异常断开
    codeDialTimeout = 504 //后端服务连接超时
)

type PemTLS struct {
    addr string
    cert_pem string
    key_pem  string
}

type LogHandle struct {
    // Log Fromat:
    // $request_time $remote_addr "$server_addr:$server_port -> $target_addr:$target_port" \
    //                           "$request" $status "$http_user_agent" $x_real_ip $http_x_forwarded_for $hashcode
    request_time     time.Duration
    remote_addr      string
    server_addr_port string
    target_addr_port string
    request          string
    status           string
    http_user_agent  string
    http_x_real_ip   string
    http_x_forwarded_for string
    hashcode         string
}

// String hashes a string to a unique hashcode.
// crc32 returns a uint32, but for our use we need
// and non negative integer. Here we cast to an integer
// and invert it if the result is negative.
func hashCode(s string) int {
    v := int(crc32.ChecksumIEEE([]byte(s)))
    if v >= 0 {
        return v
    }
    if -v >= 0 {
        return -v
    }
    // v == MinInt
    return 0
}

// Strings hashes a list of strings to a unique hashcode.
func hashCodes(strings []string) string {
    var buf bytes.Buffer

    for _, s := range strings {
        buf.WriteString(fmt.Sprintf("%s-", s))
    }
    return fmt.Sprintf("%d", hashCode(buf.String()))
}

// Ternary operation modle
func If(cond bool, a, b interface{}) interface{} {
    if cond {
        return a
    }
    return b
}

func version() {
    fmt.Printf("WSproxy v%s\n", __VERSION__)
}

func init() {
    // Default attach console, detach console
    logger.Detach("console")
    consoleConfig := &gologger.ConsoleConfig{
        Color: false,
        JsonFormat: false, 
        Format: "%timestamp_format% [%level_string%] %body%",
    }
    
    logger.Attach("console", gologger.LOGGER_LEVEL_DEBUG, consoleConfig)
    logger.SetAsync()

    // Help flag list
    var secret string
    flag.StringVar(&secret, "secret", "", "The passphrase used to decrypt target server address")
    flag.StringVar(&cfgGatewayAddr, "addr", cfgGatewayAddr, "Network address for gateway")
    flag.UintVar(&cfgDialTimeout, "timeout", cfgDialTimeout, "Timeout seconds when dial to targer server")
    flag.UintVar(&cfgBufferSize, "buffer", cfgBufferSize, "Buffer size for ReadBuffer()/WriteBuffer()")
    flag.StringVar(&cfgFormSplit, "fsplit", cfgFormSplit, "Split token from formValue, like '?t=xeR7LpmprJS8U...?v=4693225'\n(Exp: -fsplit \"?v=\",0 )  Res: 'xeR7LpmprJS8U...' ")
    flag.StringVar(&cfgFormKey, "frkey", cfgFormKey, "Key name for URL request. like '/?token=xeR7LpmprJS8U...'\n(Exp: -frkey token or -frkey token123) Fmt: ^[a-z]+[0-9]* ")
    flag.StringVar(&cfgCertFile, "ssl_cert", cfgCertFile, "SSL certificate file")
    flag.StringVar(&cfgKeyFile, "ssl_key", cfgKeyFile, "SSL key file (if separate from cert)")
    flag.BoolVar(&sslOnly, "ssl_only", false, "Run WSproxy for TLS version")
    flag.BoolVar(&appVersion, "version", false, "Print WSproxy version")
    
    flag.Parse()
    cfgSecret = string(secret)
    cfgDialTimeout = uint(time.Second) * cfgDialTimeout
    cfgFormSplit = strings.Replace(cfgFormSplit, " ", "", -1)
    cfgFormKey = strings.TrimSpace(cfgFormKey)
    cfgFormKey = strings.ToLower(cfgFormKey)
    tlsh = PemTLS{cfgGatewayAddr, cfgCertFile, cfgKeyFile}
}

// listen http
func listener() {
    err := http.ListenAndServe(cfgGatewayAddr, nil)
    if err != nil {
        fmt.Println("")
        fmt.Printf("WS-Server Listen Err: \"%s\"\n", err.Error())
        os.Exit(1)
    }
}

// listen https
func listenerTLS() {
    err := http.ListenAndServeTLS(tlsh.addr, tlsh.cert_pem, tlsh.key_pem, nil)
    if err != nil {
        fmt.Println("")
        fmt.Printf("WSS-Server Listen Err: \"%s\"\n", err.Error())
        os.Exit(1)
    }
}


func main() {
    num = 0
    
    if appVersion == true {
       version()
       return
    }

    if len(cfgSecret) == 0 {
		fmt.Printf("Missing passphrase.\n\n")
        flag.Usage()
		return
	}
    
    if cfgFormSplit != "" {
        if len(strings.Split(cfgFormSplit, ",")) == 2 {
            cfgLfSplit = strings.Split(cfgFormSplit, ",")[0]
            if cfgLfSplit == "" {
                fmt.Printf("Missing passphrase, maybe '-fsplit' format error. (Exp: -fsplit \"?v=\",0 )\n\n")
                return
            }
            _i,_ := strconv.Atoi(strings.Split(cfgFormSplit, ",")[1])
            cfgRfSplit = _i
        }else{
            fmt.Printf("Missing passphrase, maybe '-fsplit' format error. (Exp: -fsplit \"?v=\",0 )\n\n")
            flag.Usage()
            return
        }
    }
    
    
    if re := regexp.MustCompile(`^[a-z]+[0-9]*`) ; ! re.MatchString(cfgFormKey) {
        fmt.Printf("Missing passphrase, maybe '-frkey' format error. (Exp: -frkey token or -frkey token123 )\n\n")
        return
    }
  
    
    pid := syscall.Getpid()
	if err := ioutil.WriteFile("gateway.pid", []byte(strconv.Itoa(pid)), 0644); err != nil {
		fmt.Printf("Can't write pid file: %s", err)
	}
	defer os.Remove("gateway.pid")

    http.HandleFunc("/status", handleStatus)
	http.HandleFunc("/", handleWs)
    
    if sslOnly == true {
        __SSL_TLS__ = "support"
        go listenerTLS()
    }else{
        go listener()
    }
    

    fmt.Printf(`============= WSproxy running: OK , [%v] =============
Version:      %s
Address:      %s
SSL/TLS:      %s
Dial timeout: %s
Buffer size:  %d
Passphrase:   %s
URLrequest:   /?%s=
Process ID:   %d`,
        time.Unix(time.Now().Unix(), 0),
        __VERSION__,
        cfgGatewayAddr,
        __SSL_TLS__,
        time.Duration(cfgDialTimeout),
        cfgBufferSize,
        cfgSecret,
        cfgFormKey,
        pid)
    fmt.Println("\n=============")
    
    logger.Flush()
    
    exitChan := make(chan os.Signal, 1)
    signal.Notify(exitChan, syscall.SIGTERM)
    signal.Notify(exitChan, syscall.SIGINT)
    <-exitChan
    fmt.Printf("WSproxy killed\n")
    
}

//用于监控运行状态
func handleStatus(w http.ResponseWriter, r *http.Request) {
	logger.Info("/status")
    
    w.Header().Set("Server", fmt.Sprintf("WSproxy v%s\n", __VERSION__))
	html := "Hello WSproxy!\n"
	_, err := w.Write([]byte(html))
	if err != nil {
		logger.Errorf("Html write err: %s", err)
	}
}


//记录一条请求日志集
func logHandle(c net.Conn, r *http.Request, remote_addr string, runTime time.Duration, stCode int, hashCode string) {
    
    var local_addr = ""
    //var remote_addr = ""
    x_real_ip := r.Header.Get("X-Real-IP")
    x_forwarded_for := r.Header.Get("X-Forwarded-For")
    user_agent := r.Header.Get("User-Agent")
    
    if c != nil {
        local_addr  = c.LocalAddr().String()
        //remote_addr = c.RemoteAddr().String()
    }
    
    logh.request_time = runTime
    logh.remote_addr,_,_ = net.SplitHostPort(r.RemoteAddr)
    logh.http_x_real_ip  = If(x_real_ip=="", "-", x_real_ip).(string)
    logh.http_x_forwarded_for = If(x_forwarded_for=="", "-", x_forwarded_for).(string)
    logh.server_addr_port = If(local_addr=="", "-:nil", local_addr).(string)
    logh.target_addr_port = If(remote_addr=="", "-:nil", remote_addr).(string)
    logh.request    = fmt.Sprintf("%s %s %s", r.Method, r.RequestURI, r.Proto)
    logh.status     = strconv.Itoa(stCode)
    logh.http_user_agent = If(user_agent=="", "-", user_agent).(string)
    logh.hashcode   = If(hashCode=="", "-", hashCode).(string)
    
    logger.Infof("%v %s \"net:%s->%s\" \"%s\" %s \"%s\" %s %s \"User-hash:%s\"", 
		logh.request_time,
		logh.remote_addr,
		logh.server_addr_port,
		logh.target_addr_port, 
		logh.request,
		logh.status,
		logh.http_user_agent,
		logh.http_x_forwarded_for,
		logh.http_x_real_ip,
		logh.hashcode)
}


func handleWs(w http.ResponseWriter, r *http.Request) {
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
        //logger.Warningf("Not a websocket handshake, %s", err)
        return
    } else if err != nil {
		logger.Warningf("webSocket upgrade err, %s", err)
		return
	}
    
	defer ws.Close()
    //==================
    
    //收到加密串进行解码
    var fromValueTrim string
    fromValueTrim = strings.Replace(r.FormValue(cfgFormKey), " ", "+", -1)
    encrypted := strings.TrimSpace(fromValueTrim)
    
    //Token切割取样,某些时候可能会带?号, 加上-fsplit可以用于切割
    if cfgFormSplit != "" {
        if strings.Contains(encrypted, cfgLfSplit) {
            encrypted = strings.Split(encrypted, cfgLfSplit)[cfgRfSplit]
        }
    }
    // AES256cbc解密算法
    raddr, err := aes256cbc.DecryptString(cfgSecret, encrypted)
    //处理掉一些加密过程中的特殊字符, 如空格 \r\n
    raddr = strings.TrimSpace(raddr)
    if err != nil {
        logger.Errorf("Decrypt an error occurred: %s, Encrypt: %s", err, encrypted)
        return
    }

    //连接后端TCP
	client, err := net.DialTimeout("tcp", raddr, time.Duration(cfgDialTimeout))
    
    //timeout
    if ne, ok := err.(net.Error); ok && ne.Timeout() {
        //logger.Warningf("tcpServer Connect to timeout %ds, %s", time.Duration(cfgDialTimeout), err)
        //504 后端服务连接超时
        go logHandle(client, r, raddr, time.Since(_t), codeDialTimeout, _h)
        return
	}
	
	if err != nil {
		//logger.Criticalf("tcpServer failed to connect, %s", err)
        //502 后端服务不可用或没响应
        go logHandle(client, r, raddr, time.Since(_t), codeDialErr, _h)
		return
	}
	
	defer client.Close()
    //==================
    
    //记录一条请求日志集
    go logHandle(client, r, raddr, time.Since(_t), codeOK, _h)
    //==================
    
    //开始代理转发数据
	go func() {
        defer func() {
			ws.Close()
			client.Close()
        }()
    
        for {
            _, message, err := ws.ReadMessage()
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
                //503 后端服务异常断开 (!=1000/1001/1005)
                if websocket.IsUnexpectedCloseError(err,
                                                    websocket.CloseNormalClosure,
                                                    websocket.CloseGoingAway, 
                                                    websocket.CloseNoStatusReceived) {

                    go logHandle(client, r, raddr, time.Since(_t), codeCloseErr, _h)
                }else{
                    //客户端主动断开
                    logger.Noticef("%v, User-hash:%s", err, _h)
                }
                break
            }

            _, err = client.Write(message)
            if err != nil {
                logger.Warningf("tcpServer write error: %s, User-hash:%s", err, _h)
                //403 后端数据同步写异常
                //go logHandle(client, r, time.Since(_t), 403)
                break
            }
        }
		
	}()

	
    //==================
    for {
        buff := make([]byte, int(cfgBufferSize))
        num, err := client.Read(buff)
        
        if err != nil {
            if err == io.EOF {
                logger.Noticef("tcpServer read error '%s', User-hash:%s", err, _h)
                //500 后端异常断开
                //go logHandle(client, r, time.Since(_t), 500)
            }
            break
        }

        err = ws.WriteMessage(websocket.BinaryMessage, buff[:num])
        if err != nil {
            logger.Errorf("webSocket write error: %s, User-hash:%s", err, _h)
            //400 WS代理同步写异常
            //go logHandle(client, r, time.Since(_t), 400)
            break
        }
	}
    //==================
    
    
}
