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

package proxy_test

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/session"
)

func Test(t *testing.T) { TestingT(t) }

type proxySuite struct{}

var _ = Suite(&proxySuite{})

// Test file transfer using the proxy.
func (t *proxySuite) TestProxyDownload(c *C) {
	// start the fetch service proxy
	ch := make(chan interface{}, 1)
	spool := c.MkDir()
	p := proxy.NewHttpProxy(5566, spool, ch)

	err := p.Start()
	c.Assert(err, IsNil)
	defer p.Stop()

	time.Sleep(1 * time.Second)

	// create a new session
	s := session.New()
	defer s.Discard()

	// download a test file
	proxyURL := url.URL{
		Scheme: "http",
		User:   url.UserPassword(s.Id, s.Pw),
		Host:   "localhost:5566",
	}

	url, err := url.Parse("https://launchpadlibrarian.net/592566337/hello_2.10-2ubuntu4_amd64.deb")
	c.Assert(err, IsNil)

	transport := &http.Transport{
		Proxy:           http.ProxyURL(&proxyURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// authorize download
	go func() {
		msg := <-ch
		auth := msg.(proxy.ProxyAuth)
		c.Assert(auth.Id, Equals, s.Id)
		c.Assert(auth.Pw, Equals, s.Pw)
		auth.Rch <- true

	}()

	req, err := http.NewRequest("GET", url.String(), nil)
	c.Assert(err, IsNil)

	resp, err := client.Do(req)
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200)

	defer resp.Body.Close()

	go func(body io.ReadCloser) {
		_, err = io.ReadAll(body)
		c.Assert(err, IsNil)
	}(resp.Body)

	// check downloaded file information
	msg := <-ch
	v := msg.(metadata.FileDownload)

	c.Assert(v.Md.Sha1.String(), Equals, "d8c1f9634007b54c1e9aa3ba3b51395b643933c3")
	c.Assert(v.Info.StatusCode, Equals, 200)
	c.Assert(v.Info.Method, Equals, "GET")
	c.Assert(v.Info.ContentType, Equals, "application/x-debian-package")

	// no handling errors
	v.Rch <- nil
}
