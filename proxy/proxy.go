// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2026 Canonical Ltd.
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
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/canonical/fetch-service/secrets"
	"github.com/canonical/fetch-service/session"
	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/transport"
	"gopkg.in/tomb.v2"

	"github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/proxy/acl"
	"github.com/canonical/fetch-service/proxy/auth"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/utils"
)

const (
	sessionIDHeader = "X-Fetch-Session-Id"
	authRealm       = "fetch-service"

	defaultInspectionTimeout = 300 * time.Second
)

// proxyData contains contextual information for request and response handlers.
type proxyData struct {
	a *metadata.Artifact // the artifact to be inspected
}

// HTTPProxy implements a proxy that inspects downloaded contents.
type HTTPProxy struct {
	port    int                      // tcp port the proxy is listening on
	ch      chan interface{}         // channel to service dispatcher
	spool   string                   // path to file spool
	proxy   *goproxy.ProxyHttpServer // proxy handler
	srv     http.Server              // server instance
	timeout time.Duration            // inspection timeout
	tomb    tomb.Tomb                // proxy service reaper
	started atomic.Bool              // true only after tomb.Go registered the serve goroutine
}

func NewHTTPProxy(port int, spool string, cert, key []byte, ch chan interface{}) (*HTTPProxy, error) {
	ca, err := CreateProxyCA(cert, key)
	if err != nil {
		return nil, err
	}
	if err = SetProxyCA(ca); err != nil {
		return nil, err
	}

	basicAuth := func(req *http.Request, user, passwd string) bool {
		req.Header.Set(sessionIDHeader, user)
		rch := make(chan bool)
		ch <- messages.ProxyAuth{
			Rch:      rch,
			ID:       user,
			Pw:       passwd,
			HostIP:   utils.ServerIP(req),
			ClientIP: utils.ClientIP(req),
			Agent:    req.UserAgent(),
		}
		return <-rch
	}

	p := HTTPProxy{
		port:    port,
		ch:      ch,
		spool:   spool,
		timeout: defaultInspectionTimeout, // XXX: make this configurable
	}

	proxy := goproxy.NewProxyHttpServer()
	//proxy.Verbose = true

	// Set up proxy authentication
	auth.ProxyBasic(proxy, authRealm, basicAuth)

	// For every incoming request, override the RoundTripper to extract connection
	// information and check ACLs.
	proxy.OnRequest().DoFunc(p.processRoundTrip)

	proxy.OnRequest().DoFunc(p.processRequest)
	proxy.OnResponse().DoFunc(p.processResponse)
	proxy.OnRequest().HandleConnectFunc(goproxy.AlwaysMitm)

	p.proxy = proxy

	return &p, nil
}

// Start runs the proxy on the specified tcp port.
func (p *HTTPProxy) Start() error {
	addr := fmt.Sprintf(":%d", p.port)
	p.srv = http.Server{Addr: addr, Handler: p.proxy}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	logger.Infof("Starting the HTTP proxy; listening on %s\n", addr)

	p.tomb.Go(func() error {
		listener := tcpKeepAliveListener{ln.(*net.TCPListener)}
		if err := p.srv.Serve(listener); err != http.ErrServerClosed && p.tomb.Err() == tomb.ErrStillAlive {
			return err
		}
		return nil
	})
	p.started.Store(true)

	return nil
}

// Stop shuts down the proxy.
func (p *HTTPProxy) Stop() error {
	logger.Infof("Shutting down the HTTP proxy...")
	// Close unblocks Serve, then Wait ensures the serving goroutine has exited.
	if err := p.srv.Close(); err != nil {
		return err
	}
	p.tomb.Kill(nil)

	// Wait for goroutine cleanup if Start registered a serving goroutine.
	// We check p.started rather than p.srv.Handler because Handler is set
	// before net.Listen — a bind failure would leave Handler non-nil with
	// no goroutine registered, causing tomb.Wait() to block forever.
	if p.started.Load() {
		p.tomb.Wait()
	}

	return nil
}

func (p *HTTPProxy) Dying() <-chan struct{} {
	return p.tomb.Dying()
}

func (p *HTTPProxy) Err() error {
	return p.tomb.Err()
}

// processRoundTrip gets destination connection information to check ACLs.
func (p *HTTPProxy) processRoundTrip(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	host, _, err := net.SplitHostPort(req.URL.Host)
	if err != nil {
		host = req.URL.Host
	}

	logger.Infof("proxy: process roundtrip: %s", req.URL.String())
	tr := transport.Transport{
		Proxy:           proxyFromEnvironment,
		TLSClientConfig: &tls.Config{ServerName: host},
	}

	ctx.RoundTripper = goproxy.RoundTripperFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Response, error) {
		details, resp, err := tr.DetailedRoundTrip(r)
		if err != nil {
			logger.Debugf("proxy: roundtrip error: %s", err)
			return resp, err
		}
		ip := details.TCPAddr.IP
		logger.Infof("proxy: request %s: IP address %s", r.URL.Host, ip.String())
		logger.Debugf("proxy: check request acls for %s", r.URL.String())
		if acl.Allowed(ip) {
			logger.Infof("proxy: access to %s allowed", ip.String())
		} else {
			logger.Infof("proxy: access to %s blocked", ip.String())
			resp = httpResponse(r, http.StatusForbidden, []byte("Access denied"))
		}
		return resp, nil
	})
	return req, nil
}

// processRequest handles HTTP requests to the server.
func (p *HTTPProxy) processRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	logger.Debugf("proxy: process request: %s", req.URL.String())
	requestHeader := copyHTTPHeader(req.Header)

	if ctx.UserData != nil {
		sessionID, ok := ctx.UserData.(string)
		if ok {
			// Set session ID in mitm requests
			//logger.Debugf("set session ID header in mitm request to %s", sessionID)
			req.Header.Set(sessionIDHeader, sessionID)
		}
	}

	sid := req.Header.Get(sessionIDHeader)

	a := metadata.NewArtifact()
	a.Request = req
	a.SessionID = sid

	url := utils.NormalizedURL(req.URL)
	a.CurrentDownload.StartTime = time.Now().UTC()
	a.CurrentDownload.URL = url
	a.CurrentDownload.Address = req.RemoteAddr
	a.CurrentDownload.Method = req.Method
	a.CurrentDownload.UserAgent = req.Header.Get("User-Agent")
	a.CurrentDownload.RequestHeader = requestHeader

	a.SetLogger(logger.NewSessionLogger(sid))

	reqInsp := messages.NewRequestInspection(a)
	p.ch <- reqInsp
	err := <-reqInsp.Rch
	if err != nil {
		a.Logger().Info(err.Error())
		return req, goproxy.NewResponse(
			req, goproxy.ContentTypeText,
			http.StatusForbidden,
			fmt.Sprintf("download authorization denied: %s", err),
		)
	}

	sl := a.Logger()
	sec := session.GetSessionSecrets(sid)
	if secrets.InjectSecrets(sec, url, req, sl) {
		sl.Debugf("applied secrets to %s", url)
	}

	req.Body, err = NewRequestHandler(req, a, p.ch)
	if err != nil {
		return req, internalErrorResponse(req, "Cannot handle requests")
	}

	ctx.UserData = proxyData{a: a}

	return req, nil
}

// processResponse handles HTTP responses from the server.
func (p *HTTPProxy) processResponse(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	if resp == nil || resp.StatusCode != http.StatusOK {
		return resp
	}
	logger.Debugf("proxy: process response: %s", resp.Request.URL.String())

	a := ctx.UserData.(proxyData).a
	sl := a.Logger()

	var err error
	body, err := NewFileDownloadHandler(resp, a, p.spool, p.ch, p.timeout)
	if err != nil {
		if a.Tempfile != "" {
			os.Remove(a.Tempfile)
		}
		if err == common.ErrRejectedArtifact {
			sl.Infof("proxy: file download not authorized: %s", err)
			return forbiddenResponse(resp.Request, "Download not authorized")
		}
		sl.Infof("proxy: file download error: %s: %s", a.Tempfile, err)
		return internalErrorResponse(resp.Request, "Cannot handle file downloads")
	}

	newResp := copyHTTPResponse(resp)
	newResp.Body = body

	return newResp
}

func httpResponse(req *http.Request, code int, msg []byte) *http.Response {
	return &http.Response{
		StatusCode:    code,
		ProtoMajor:    1,
		ProtoMinor:    1,
		Request:       req,
		Header:        map[string][]string{"Content-Type": []string{"text/plain"}},
		Body:          io.NopCloser(bytes.NewBuffer(msg)),
		ContentLength: int64(len(msg)),
	}

}

func proxyFromEnvironment(req *http.Request) (*url.URL, error) {
	if shouldBypassProxy(req.URL.Hostname()) {
		return nil, nil
	}

	var proxy string
	switch req.URL.Scheme {
	case "https":
		proxy = getenvAny("HTTPS_PROXY", "https_proxy")
	case "http":
		proxy = getenvAny("HTTP_PROXY", "http_proxy")
	}

	if proxy == "" {
		proxy = getenvAny("ALL_PROXY", "all_proxy")
	}

	if proxy == "" {
		return nil, nil
	}

	parsedProxy, err := url.Parse(proxy)
	if err != nil {
		return nil, err
	}
	redactedProxy := *parsedProxy
	if redactedProxy.User != nil {
		redactedProxy.User = url.UserPassword("xxxx", "xxxx")
	}

	logger.Infof("proxy: using upstream %s proxy: %s", req.URL.Scheme, redactedProxy.String())

	return parsedProxy, nil
}

// shouldBypassProxy checks if the given host can do requests directly
// to the destination server. There's no formal spec for this feature,
// but mainstream applications commonly accept a comma-separated list
// of values, with exact matches making the host to skip the proxy.
// Suffix or glob matching is also commonly accepted. We'll use glob
// to make matching safer and more predictable.
func shouldBypassProxy(host string) bool {
	noProxy := getenvAny("NO_PROXY", "no_proxy")
	if noProxy == "" {
		return false
	}

	host = strings.ToLower(host)
	for _, h := range strings.Split(noProxy, ",") {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}

		h = strings.ToLower(h)
		if host == h {
			return true
		}

		ok, err := path.Match(h, host)
		if err == path.ErrBadPattern {
			logger.Warningf("proxy: bypass pattern %q is malformed", h)
		}
		if ok {
			return true
		}
	}
	return false
}

func getenvAny(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
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

	err = tc.SetKeepAlive(true)
	if err != nil {
		return nil, err
	}

	err = tc.SetKeepAlivePeriod(3 * time.Minute)
	if err != nil {
		return nil, err
	}

	return tc, nil
}

func internalErrorResponse(r *http.Request, msg string) *http.Response {
	return goproxy.NewResponse(r, goproxy.ContentTypeText, http.StatusInternalServerError, msg)
}

func forbiddenResponse(r *http.Request, msg string) *http.Response {
	return goproxy.NewResponse(r, goproxy.ContentTypeText, http.StatusForbidden, msg)
}

// copyHTTPHeader deepcopies HTTP header maps.
func copyHTTPHeader(h http.Header) http.Header {
	c := make(http.Header, len(h))
	for k, v := range h {
		val := make([]string, len(v))
		copy(val, v)
		c[k] = val
	}
	return c
}

// copyHTTPResponse deepcopies HTTP responses.
func copyHTTPResponse(r *http.Response) *http.Response {
	if r == nil {
		return nil
	}

	newResp := http.Response{
		Status:           r.Status,
		StatusCode:       r.StatusCode,
		Proto:            r.Proto,
		ProtoMajor:       r.ProtoMajor,
		ProtoMinor:       r.ProtoMinor,
		Header:           copyHTTPHeader(r.Header),
		Body:             nil,
		ContentLength:    r.ContentLength,
		TransferEncoding: slices.Clone(r.TransferEncoding),
		Close:            r.Close,
		Uncompressed:     r.Uncompressed,
		Trailer:          copyHTTPHeader(r.Trailer),
		Request:          r.Request,
		TLS:              r.TLS,
	}

	return &newResp
}
