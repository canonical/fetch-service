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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
	"github.com/canonical/fetch-service/testutils"
	"github.com/canonical/fetch-service/utils"
)

func Test(t *testing.T) { TestingT(t) }

type proxySuite struct{}

var _ = Suite(&proxySuite{})

func (t *proxySuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

func (t *proxySuite) TestServerError(c *C) {
	ch := make(chan interface{}, 1)
	spool := c.MkDir()
	p, err := proxy.NewHttpProxy(5566, spool, testutils.ProxyCert, testutils.ProxyKey, ch)
	c.Assert(err, IsNil)

	err = errors.New("an error")
	p.ForceError(err)
	c.Assert(p.Err(), Equals, err)
}

// Test file transfer using the proxy.
func (t *proxySuite) TestProxyDownload(c *C) {
	// start the fetch service proxy
	ch := make(chan interface{}, 1)
	spool := c.MkDir()
	p, err := proxy.NewHttpProxy(5566, spool, testutils.ProxyCert, testutils.ProxyKey, ch)
	c.Assert(err, IsNil)

	err = p.Start()
	c.Assert(err, IsNil)
	defer func() {
		err := p.Stop()
		c.Assert(err, IsNil)
	}()

	time.Sleep(1 * time.Second)

	// create a new session
	s := session.New(spool, 0, true)
	defer s.Discard()

	// download a test file
	proxyURL := url.URL{
		Scheme: "http",
		User:   url.UserPassword(s.Id, s.Token),
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

	go func() {
		req, err := http.NewRequest("GET", url.String(), nil)
		c.Assert(err, IsNil)

		resp, err := client.Do(req)
		c.Assert(err, IsNil)
		c.Assert(resp.StatusCode, Equals, 200)
	}()

	// authorize download
	msg := <-ch
	auth := msg.(messages.ProxyAuth)
	c.Assert(auth.Id, Equals, s.Id)
	c.Assert(auth.Pw, Equals, s.Token)
	auth.Rch <- true

	// run request inspectors
	msg = <-ch
	v := msg.(messages.RequestInspection)
	v.Rch <- nil // no errors

	// artifact downloaded
	msg = <-ch
	u := msg.(messages.ResponseInspection)

	dest := filepath.Join(u.A.AssetDir, fmt.Sprintf("%s.data", u.A.Metadata.Sha256))
	err = os.MkdirAll(filepath.Dir(dest), 0755)
	c.Assert(err, IsNil)

	err = utils.MoveFile(u.A.Tempfile, dest)
	c.Assert(err, IsNil)
	os.Remove(u.A.Tempfile)

	// check downloaded file information
	c.Assert(v.A.MetadataVersion, Equals, "0.2")
	c.Assert(u.A.Metadata.Sha1.String(), Equals, "d8c1f9634007b54c1e9aa3ba3b51395b643933c3")
	c.Assert(u.A.Metadata.Sha256.String(), Equals, "750335248ccc68d07397e2b843d94fd1a164ddeca23892ca8398b5d528cd89eb")
	c.Assert(u.A.Metadata.Size, Equals, int64(26600))

	dl := u.A.CurrentDownload
	c.Assert(dl.StatusCode, Equals, 200)
	c.Assert(dl.URL, Equals, "https://launchpadlibrarian.net:443/592566337/hello_2.10-2ubuntu4_amd64.deb")
	c.Assert(dl.Method, Equals, "GET")
	c.Assert(dl.ContentType, Equals, "application/x-debian-package")
	c.Assert(dl.UserAgent, Equals, "Go-http-client/1.1")
	c.Assert(dl.RequestHeader, DeepEquals, map[string][]string{
		"Accept-Encoding": []string{"gzip"},
		"User-Agent":      []string{"Go-http-client/1.1"},
	})
	c.Assert(dl.ResponseHeader["Content-Length"], DeepEquals, []string{"26600"})
	c.Assert(dl.ResponseHeader["Content-Type"], DeepEquals, []string{"application/x-debian-package"})

	u.Rch <- nil // no errors
}

func (t *proxySuite) TestCopyHeader(c *C) {
	for _, tc := range []struct {
		key string
		val []string
	}{
		{"key", []string{}},
		{"key", []string{"a", "b", "c"}},
	} {
		data := map[string][]string{tc.key: tc.val}
		newData := proxy.CopyHttpHeader(data)
		delete(data, tc.key)
		c.Assert(data[tc.key], IsNil)
		c.Assert(newData, Not(Equals), data)
		c.Assert(newData[tc.key], DeepEquals, tc.val)
		c.Assert(newData[tc.key], Not(Equals), tc.val)
	}
}
