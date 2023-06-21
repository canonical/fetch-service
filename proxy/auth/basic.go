// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2012 Elazar Leibovich.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are
 * met:
 *
 *    - Redistributions of source code must retain the above copyright
 * notice, this list of conditions and the following disclaimer.
 *    - Redistributions in binary form must reproduce the above
 * copyright notice, this list of conditions and the following disclaimer
 * in the documentation and/or other materials provided with the
 * distribution.
 *    - Neither the name of Elazar Leibovich. nor the names of its
 * contributors may be used to endorse or promote products derived from
 * this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

/*
 * Changes for use in the fetch service:
 * - pass the request instance to the authentication function
 * - allow authentication to be used with the mitm connect handler
 *   (see https://github.com/elazarl/goproxy/pull/428/files)
 */

package auth

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/elazarl/goproxy"
)

var unauthorizedMsg = []byte("407 Proxy Authentication Required")

func BasicUnauthorized(req *http.Request, realm string) *http.Response {
	// TODO(elazar): verify realm is well formed
	return &http.Response{
		StatusCode: 407,
		ProtoMajor: 1,
		ProtoMinor: 1,
		Request:    req,
		Header: http.Header{
			"Proxy-Authenticate": []string{"Basic realm=" + realm},
			"Proxy-Connection":   []string{"close"},
		},
		Body:          ioutil.NopCloser(bytes.NewBuffer(unauthorizedMsg)),
		ContentLength: int64(len(unauthorizedMsg)),
	}
}

var proxyAuthorizationHeader = "Proxy-Authorization"

func auth(req *http.Request, f func(req *http.Request, user, passwd string) bool, ctx *goproxy.ProxyCtx) bool {
	authheader := strings.SplitN(req.Header.Get(proxyAuthorizationHeader), " ", 2)
	req.Header.Del(proxyAuthorizationHeader)
	if len(authheader) != 2 || authheader[0] != "Basic" {
		return false
	}
	userpassraw, err := base64.StdEncoding.DecodeString(authheader[1])
	if err != nil {
		return false
	}
	userpass := strings.SplitN(string(userpassraw), ":", 2)
	if len(userpass) != 2 {
		return false
	}
	if ctx != nil {
		ctx.UserData = userpass[0]
	}
	return f(req, userpass[0], userpass[1])
}

// Basic returns a basic HTTP authentication handler for requests
//
// You probably want to use auth.ProxyBasic(proxy) to enable authentication for all proxy activities
func Basic(realm string, f func(req *http.Request, user, passwd string) bool) goproxy.ReqHandler {
	return goproxy.FuncReqHandler(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if ctx.UserData == nil && !auth(req, f, nil) {
			return nil, BasicUnauthorized(req, realm)
		}
		return req, nil
	})
}

// BasicConnect returns a basic HTTP authentication handler for CONNECT requests
//
// You probably want to use auth.ProxyBasic(proxy) to enable authentication for all proxy activities
func BasicConnect(realm string, f func(req *http.Request, user, passwd string) bool) goproxy.HttpsHandler {
	return goproxy.FuncHttpsHandler(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		if ctx.UserData == nil && !auth(ctx.Req, f, ctx) {
			ctx.Resp = BasicUnauthorized(ctx.Req, realm)
			return goproxy.RejectConnect, host
		}
		return nil, host
	})
}

// ProxyBasic will force HTTP authentication before any request to the proxy is processed
func ProxyBasic(proxy *goproxy.ProxyHttpServer, realm string, f func(req *http.Request, user, passwd string) bool) {
	proxy.OnRequest().Do(Basic(realm, f))
	proxy.OnRequest().HandleConnect(BasicConnect(realm, f))
}
