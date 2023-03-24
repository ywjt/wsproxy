// Copyright 2023 The WebSocket Proxy Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// @Author: YWJT / ZhiQiang Koo
// @Modify: 2023-03-24
//

package main

import (
	"fmt"
    "strings"
    "time"
    "flag"
    "strconv"
    "regexp"
    "github.com/google/uuid"
)


var (
    new_addr   string
    cfgSecret  string
    cfgLfSplit string
    cfgRfSplit int
    serverUUID     = uuid.New().String()
    
    cfgGatewayAddr = "0.0.0.0:1443"
    cfgDialTimeout = uint(3)
    cfgBufferSize  = uint(1 * 1024)
    cfgMaxConns    = uint(64 * 1024)
    cfgBuffFormat  = "bin"  // {bin, text}
    cfgFormSplit   = ""
    cfgFormKey     = "token"
    
    cfgCertFile  = "./cert.pem"
    cfgKeyFile   = "./key.pem"
    appVersion  = true
    sslOnly     = true
    aesOnly     = true
    PProto      = true
    
    __SSL_TLS__ = "No support (no cert file)"
    __PPROTO__ = "disable"
)

func version() {
    fmt.Printf("WSproxy v%s\n", __VERSION__)
}

func init() {

    // Help flag list
    var secret string
	flag.StringVar(&secret, "secret", "", "The passphrase used to decrypt target server address")
	flag.StringVar(&cfgGatewayAddr, "addr", cfgGatewayAddr, "Network address for gateway")
	flag.UintVar(&cfgDialTimeout, "timeout", cfgDialTimeout, "Timeout seconds when dial to targer server")
    flag.UintVar(&cfgBufferSize, "buffer", cfgBufferSize, "Buffer size for ReadBuffer()/WriteBuffer()")
    flag.UintVar(&cfgMaxConns, "max_conns", cfgMaxConns, "Max connections to slots available.")
    flag.StringVar(&cfgBuffFormat, "stream", cfgBuffFormat, "Buffer stream format for (text, bin). Only TCP/UDP backend.\n(Exp: -stream bin or -stream text )")
    flag.StringVar(&cfgFormSplit, "fsplit", cfgFormSplit, "Split token from formValue, like '?t=xeR7LpmprJS8U...?v=4693225'\n(Exp: -fsplit \"?v=\",0 )  Res: 'xeR7LpmprJS8U...' ")
    flag.StringVar(&cfgFormKey, "frkey", cfgFormKey, "Key name for URL request. like '/?token=xeR7LpmprJS8U...'\n(Exp: -frkey token or -frkey token123) Fmt: ^[a-z]+[0-9]* ")
    flag.StringVar(&cfgCertFile, "ssl_cert", cfgCertFile, "SSL certificate file")
	flag.StringVar(&cfgKeyFile, "ssl_key", cfgKeyFile, "SSL key file (if separate from cert)")
    flag.BoolVar(&sslOnly, "ssl_only", false, "Run WSproxy for TLS version")
    flag.BoolVar(&aesOnly, "aes_only", false, "Run WSproxy on encryption mode for AES")
    flag.BoolVar(&PProto, "proxyproto", false, "Enable proxy protocol mode, Requires backend server support")
    flag.BoolVar(&appVersion, "version", false, "Print WSproxy version")
    
	flag.Parse()
    cfgSecret = string(secret)
	cfgDialTimeout = uint(time.Second) * cfgDialTimeout
    cfgFormSplit = strings.Replace(cfgFormSplit, " ", "", -1)
    cfgFormKey = strings.TrimSpace(cfgFormKey)
    cfgFormKey = strings.ToLower(cfgFormKey)
    cfgBuffFormat = strings.ToLower(cfgBuffFormat)
    __SSL_TLS__ = If(sslOnly==true, "support", __SSL_TLS__).(string)
    __PPROTO__ = If(PProto==true, "enable", __PPROTO__).(string)
}

func main() {

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
    
    if cfgBuffFormat != "bin" && cfgBuffFormat != "text" {
        fmt.Printf("Missing passphrase, '-stream %s' No support.\n", cfgBuffFormat)
        return
    }
  
    //set max connectionns
    setMaxConns(int(cfgMaxConns))

    pid := NewSignal()
    runInfo := fmt.Sprintf(`============= WSproxy running: OK , [%v] =============
UUID:          %s
Version:       %s
Address:       %s
SSL/TLS:       %s
Proxy Proto:   %s
Dial Timeout:  %s
Max Connects:  %d
Buffer Size:   %d
Passphrase:    %s
URL Request:   /?%s=
Process ID:    %d
=============
`,
        time.Unix(time.Now().Unix(), 0),
        serverUUID,
        __VERSION__,
        cfgGatewayAddr,
        __SSL_TLS__,
        __PPROTO__,
        time.Duration(cfgDialTimeout),
        cfgMaxConns,
        cfgBufferSize,
        cfgSecret,
        cfgFormKey,
        pid)
    
    logger.Flush()
    
    //runtime.GOMAXPROCS(runtime.NumCPU())
    NewServer(cfgGatewayAddr, 
              cfgCertFile, 
               cfgKeyFile, 
                  sslOnly, 
                  runInfo).start()
    
}
