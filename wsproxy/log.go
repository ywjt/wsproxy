// Copyright 2023 The WebSocket Proxy Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// @Author: YWJT / ZhiQiang Koo
// @Modify: 2023-03-03  
//

package main

import (
    "net"
	"net/http"
    "gologger"
	"fmt"
    "time"
    "strconv"
)

var logger = gologger.NewLogger()

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

}

type LogStruck struct {
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

//记录一条请求日志集
func (logh *LogStruck) Out() {
    logger.Infof("%v %s \"net:%s->%s\" \"%s\" %s \"%s\" %s %s \"User-Id:%s\"", 
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


func log(c net.Conn, r *http.Request, raddr string, runTime time.Duration, stCode int, hashCode string) *LogStruck {
    var local_addr = ""
    //var remote_addr = ""
    x_real_ip := r.Header.Get("X-Real-IP")
    x_forwarded_for := r.Header.Get("X-Forwarded-For")
    user_agent := r.Header.Get("User-Agent")
    
    if c != nil {
        local_addr  = c.LocalAddr().String()
        //remote_addr = c.RemoteAddr().String()
    }
    
    request_time := runTime
    remote_addr,_,_ := net.SplitHostPort(r.RemoteAddr)
    http_x_real_ip  := If(x_real_ip=="", "-", x_real_ip).(string)
    http_x_forwarded_for := If(x_forwarded_for=="", "-", x_forwarded_for).(string)
    server_addr_port := If(local_addr=="", "-:nil", local_addr).(string)
    target_addr_port := If(raddr=="", "-:nil", raddr).(string)
    request    := fmt.Sprintf("%s %s %s", r.Method, r.RequestURI, r.Proto)
    status     := strconv.Itoa(stCode)
    http_user_agent := If(user_agent=="", "-", user_agent).(string)
    hashcode   := If(hashCode=="", "-", hashCode).(string)
    
    return &LogStruck{request_time, 
                       remote_addr, 
                       server_addr_port, 
                       target_addr_port, 
                       request, status, 
                       http_user_agent, 
                       http_x_real_ip, 
                       http_x_forwarded_for, 
                       hashcode}

}
