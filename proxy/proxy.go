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
	"log"
	"net"
	"net/http"
	"time"

	"github.com/elazarl/goproxy"
)

// DownloadInfo holds information about each downloaded artifact.
type DownloadInfo struct {
	StatusCode  int
	Status      string
	Method      string
	URL         string
	ContentType string
	UserAgent   string
	// Size
	// Digest
}

// proxyData contains contextual information for request and response handlers.
type proxyData struct {
	ch chan interface{} // channel to send messages back to the service dispacher
}

// HttpProxy implements a proxy that inspects downloaded contents.
type HttpProxy struct {
	port  int                      // tcp port the proxy is listening on
	proxy *goproxy.ProxyHttpServer // proxy handler
	srv   http.Server              // server instance
}

func NewHttpProxy(port int, ch chan interface{}) *HttpProxy {
	processRequest := func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		ctx.UserData = &proxyData{ch}
		return req, nil
	}

	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().DoFunc(processRequest)
	proxy.OnResponse().DoFunc(processResponse)
	proxy.OnRequest().HandleConnectFunc(goproxy.AlwaysMitm)

	p := HttpProxy{port: port, proxy: proxy}

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
	log.Printf("listening on %s\n", addr)

	go func() {
		err := p.srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
		if err != http.ErrServerClosed {
			log.Fatalf("cannot start server: %v", err)
		}
	}()

	return nil
}

// Stop shuts down the proxy.
func (p *HttpProxy) Stop() {
	log.Printf("shutting down...")
	p.srv.Close()
}

// processResponse handles HTTP responses from the server.
func processResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	data := ctx.UserData.(*proxyData)

	info := DownloadInfo{
		StatusCode:  resp.StatusCode,
		Status:      resp.Status,
		Method:      resp.Request.Method,
		URL:         resp.Request.URL.String(),
		ContentType: resp.Header.Get("Content-Type"),
	}

	data.ch <- info

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
