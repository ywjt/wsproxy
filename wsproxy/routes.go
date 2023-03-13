// Copyright 2023 The WebSocket Proxy Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// @Author: YWJT / ZhiQiang Koo
// @Modify: 2023-03-13  
//

package main

import (
    "fmt"
	"net/http"
)

func init() {
    //run http normal
    http.HandleFunc("/status", url_status)
    http.HandleFunc("/ok", url_check)
    //...
}


//monitor
func url_status(w http.ResponseWriter, r *http.Request) {
    logger.Info(r.URL.String())
    w.Header().Set("Server", fmt.Sprintf("WSproxy v%s\n", __VERSION__))
	//html := "====== Hello WSproxy! ======\n" + fmt.Sprintf("Conns available: %v\n", len(pool))
    html := fmt.Sprintf(`====== Hello WSproxy! ======
    UUID: %s
    Conns available: %v
    `, 
      serverUUID,
      len(pool))
    
	_, err := w.Write([]byte(html))
	if err != nil {
		logger.Errorf("Html write err: %s", err)
	}
}

//check
func url_check(w http.ResponseWriter, r *http.Request) {
    logger.Info(r.URL.String())
    w.Header().Set("Server", fmt.Sprintf("WSproxy v%s\n", __VERSION__))
	html := "WSproxy is OK!"
    
	_, err := w.Write([]byte(html))
	if err != nil {
		logger.Errorf("Html write err: %s", err)
	}
}
