// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023 Canonical Ltd.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package proxy

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/elazarl/goproxy"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/proxy/auth"
	"github.com/canonical/fetch-service/service/messages"
)

const (
	sessionIdHeader = "X-Fetch-Session-Id"
	authRealm       = "fetch-service"
)

// ProxyAuth contains credentials for basic authentication.
type ProxyAuth struct {
	Rch chan bool // return channel
	Id  string    // user (session id)
	Pw  string    // password
}

// proxyData contains contextual information for request and response handlers.
type proxyData struct {
	a *metadata.Artefact
}

// HttpProxy implements a proxy that inspects downloaded contents.
type HttpProxy struct {
	port  int                      // tcp port the proxy is listening on
	ch    chan interface{}         // channel to service dispatcher
	spool string                   // path to file spool
	proxy *goproxy.ProxyHttpServer // proxy handler
	srv   http.Server              // server instance
}

func NewHttpProxy(port int, spool string, ch chan interface{}) *HttpProxy {
	basicAuth := func(req *http.Request, user, passwd string) bool {
		logger.Debugf("set session ID header in request to %s", user)
		req.Header.Set(sessionIdHeader, user)
		rch := make(chan bool)
		ch <- ProxyAuth{rch, user, passwd}
		return <-rch
	}

	p := HttpProxy{port: port, ch: ch, spool: spool}

	proxy := goproxy.NewProxyHttpServer()
	//proxy.Verbose = true

	// Set up proxy authentication
	auth.ProxyBasic(proxy, authRealm, basicAuth)

	proxy.OnRequest().DoFunc(p.processRequest)
	proxy.OnResponse().DoFunc(p.processResponse)
	proxy.OnRequest().HandleConnectFunc(goproxy.AlwaysMitm)

	p.proxy = proxy

	return &p
}

// Start runs the proxy on the specified tcp port.
func (p *HttpProxy) Start() error {
	addr := fmt.Sprintf(":%d", p.port)
	p.srv = http.Server{Addr: addr, Handler: p.proxy}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	logger.Infof("listening on %s\n", addr)

	go func() {
		err := p.srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
		if err != http.ErrServerClosed {
			logger.Fatalf("cannot start server: %v", err)
		}
	}()

	return nil
}

// Stop shuts down the proxy.
func (p *HttpProxy) Stop() {
	logger.Infof("shutting down...")
	p.srv.Close()
}

// processRequest handles HTTP requests to the server.
func (p *HttpProxy) processRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	if ctx.UserData != nil {
		sessionId, ok := ctx.UserData.(string)
		if ok {
			// Set session ID in mitm requests
			logger.Debugf("set session ID header in mitm request to %s", sessionId)
			req.Header.Set(sessionIdHeader, sessionId)
		}
	}

	a := metadata.NewArtefact()
	a.SessionId = req.Header.Get(sessionIdHeader)

	a.CurrentDownload.StartTime = time.Now().UTC()
	a.CurrentDownload.URL = req.URL.String()
	a.CurrentDownload.Address = req.RemoteAddr
	a.CurrentDownload.Method = req.Method
	a.CurrentDownload.UserAgent = req.Header.Get("User-Agent")

	authReq := messages.NewRequestAuthorization(a)
	p.ch <- authReq
	err := <-authReq.Rch
	if err != nil {
		return req, goproxy.NewResponse(
			req, goproxy.ContentTypeText,
			http.StatusForbidden,
			fmt.Sprintf("download authorization denied: %s", err),
		)
	}

	req.Body, err = NewRequestHandler(req, a, p.ch)
	if err != nil {
		return req, internalErrorResponse(req, "Cannot handle requests")
	}

	ctx.UserData = proxyData{a: a}

	return req, nil
}

// processResponse handles HTTP responses from the server.
func (p *HttpProxy) processResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if resp == nil || resp.StatusCode != http.StatusOK {
		return resp
	}

	a := ctx.UserData.(proxyData).a

	var err error
	resp.Body, err = NewFileDownloadHandler(resp, a, p.spool, p.ch)
	if err != nil {
		logger.Warningf(err.Error())
		return internalErrorResponse(resp.Request, "Cannot handle file downloads")
	}

	return resp
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

func internalErrorResponse(r *http.Request, msg string) *http.Response {
	return goproxy.NewResponse(r, goproxy.ContentTypeText, http.StatusInternalServerError, msg)
}
