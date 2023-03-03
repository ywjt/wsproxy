// Copyright 2023 The WebSocket Proxy Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// @Author: YWJT / ZhiQiang Koo
// @Modify: 2023-02-21   
//

package main


// Ternary operation modle
func If(cond bool, a, b interface{}) interface{} {
    if cond {
        return a
    }
    return b
}