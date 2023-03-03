// Copyright 2023 The WebSocket Proxy Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// @Author: YWJT / ZhiQiang Koo
// @Modify: 2023-03-03   
//

package main

import (
    "fmt"
    "context"
	"net/http"
	"os"
	"os/signal"
    "syscall"
    "io/ioutil"
    "strconv"
)


type Server struct {
    cert string
    key  string
    tls_mod  bool
    srv *http.Server
    info string
}

// 创建一个server的接口
func NewServer(ip_port string, cert_pem string, key_pem string, tls_mod bool, info string) *Server {
    server := &Server{
        cert: cert_pem,
        key:  key_pem,
        tls_mod: tls_mod,
        srv: &http.Server{Addr: ip_port},
        info: info,
    }
    return server
}

// 启动服务器的接口
func (s *Server) start() {
	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
        signal.Notify(sigint, syscall.SIGTERM)
        signal.Notify(sigint, syscall.SIGINT)
		<-sigint

		// We received an interrupt signal, shut down.
		if err := s.srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			fmt.Printf("HTTP server Shutdown: %v\n", err)
            CloseSignal()
		}
		close(idleConnsClosed)
        fmt.Printf("WSproxy killed\n")
	}()

    if s.tls_mod {
        go s.l_https()
    }else{
        go s.l_http()
    }
    
    fmt.Printf(s.info)
	<-idleConnsClosed
}


// listen http
func (s *Server) l_http() {
    if err := s.srv.ListenAndServe(); err != http.ErrServerClosed {
        fmt.Println("")
		fmt.Printf("HTTP Server Listen Err: \"%s\"\n", err.Error())
        CloseSignal()
	}
}

// listen https
func (s *Server) l_https() {
    if err := s.srv.ListenAndServeTLS(s.cert, s.key); err != http.ErrServerClosed {
        fmt.Println("")
		fmt.Printf("HTTPS Server Listen Err: \"%s\"\n", err.Error())
        CloseSignal()
	}
}


func NewSignal() int {
    pid := syscall.Getpid()
    if err := ioutil.WriteFile("gateway.pid", []byte(strconv.Itoa(pid)), 0644); err != nil {
        fmt.Printf("Can't write pid file: %s", err)
    }
    defer os.Remove("gateway.pid")
    return pid
}

func CloseSignal() {
    defer os.Remove("gateway.pid")
    logger.Flush()
    os.Exit(1)
}
