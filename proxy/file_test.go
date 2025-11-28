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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/utils"
)

const (
	MySha256 = "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950"
)

type fileSuite struct{}

func (t *fileSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&fileSuite{})

func (t *fileSuite) TestNewFileDownloadHandler(c *C) {
	ch := make(chan interface{}, 1)
	body := io.NopCloser(bytes.NewBufferString("Request body"))
	req, err := http.NewRequest("GET", "http://foo/bar", body)
	c.Assert(err, IsNil)

	req.Header.Set("User-Agent", "test/1.0")
	req.Header.Set("X-Fetch-Session-Id", "1234")

	spoolDir := c.MkDir()

	// download the file
	resp := &http.Response{
		StatusCode: 200,
		Status:     "200 'Tis good",
		Request:    req,
		Body:       io.NopCloser(bytes.NewBufferString("Response body")),
		Header:     http.Header{"Content-Type": []string{"application/x-test"}},
	}

	a := metadata.NewArtifact()
	a.CurrentDownload.URL = req.URL.String()
	a.CurrentDownload.Address = req.RemoteAddr
	a.CurrentDownload.Method = req.Method

	testSync := make(chan struct{})

	go func() {
		defer func() {
			testSync <- struct{}{}
		}()
		var err error
		_, err = proxy.NewFileDownloadHandler(resp, a, spoolDir, ch, 60*time.Second)
		c.Assert(err, IsNil)
	}()

	msg := <-ch
	v := msg.(messages.ResponseInspection)

	c.Assert(a.CacheDir(), Equals, filepath.Join(spoolDir, "1234", "cache"))

	// check file metadata
	c.Assert(v.A.Metadata.Sha1.String(), Equals, "176070ca20a7563bed4cef2212a9be37af09f14a")
	c.Assert(v.A.Metadata.Sha256.String(), Equals, "f736153d1508e544b6c5ea19e3c2b7448d9af33608d195195e748cb54965e61b")
	c.Assert(v.A.Metadata.Size, Equals, int64(13))

	// check download info
	dl := v.A.CurrentDownload
	c.Assert(dl.StatusCode, Equals, 200)
	c.Assert(dl.Status, Equals, "200 'Tis good")
	c.Assert(dl.Method, Equals, "GET")
	c.Assert(dl.URL, Equals, "http://foo/bar")
	c.Assert(dl.ContentType, Equals, "application/x-test")
	c.Assert(dl.ResponseHeader["Content-Type"][0], Equals, "application/x-test")
	c.Assert(dl.Sha256.String(), Equals, "f736153d1508e544b6c5ea19e3c2b7448d9af33608d195195e748cb54965e61b")

	// We're the service dispatcher, so we need to rename the temporary file
	assetsDir := filepath.Join(spoolDir, "1234", "assets")
	err = os.MkdirAll(assetsDir, 0755)
	c.Assert(err, IsNil)

	assetName := filepath.Join(assetsDir, fmt.Sprintf("%s.data", a.Metadata.Sha256))
	err = utils.MoveFile(a.Tempfile, assetName)
	c.Assert(err, IsNil)

	// Send fake inspection result to the response channel. The file download handler
	// will open the downloaded file for further processing.
	v.Rch <- nil

	// Wait for the goroutine to finish before deleting spoolDir.
	<-testSync

	// download it again
	req, err = http.NewRequest("POST", "http://different/url", body)
	c.Assert(err, IsNil)

	req.Header.Set("User-Agent", "test/1.0")
	req.Header.Set("X-Fetch-Session-Id", "1234")

	resp = &http.Response{
		StatusCode: 200,
		Status:     "200 Still good",
		Request:    req,
		Body:       io.NopCloser(bytes.NewBufferString("Response body")), // same content
	}

	a = metadata.NewArtifact()
	a.CurrentDownload.URL = req.URL.String()
	a.CurrentDownload.Address = req.RemoteAddr
	a.CurrentDownload.Method = req.Method

	go func() {
		defer func() {
			testSync <- struct{}{}
		}()
		var err error
		_, err = proxy.NewFileDownloadHandler(resp, a, spoolDir, ch, 60*time.Second)
		c.Assert(err, IsNil)
	}()

	msg = <-ch
	v = msg.(messages.ResponseInspection)

	// check download info
	dl = v.A.CurrentDownload
	c.Assert(dl.StatusCode, Equals, 200)
	c.Assert(dl.Status, Equals, "200 Still good")
	c.Assert(dl.Method, Equals, "POST")
	c.Assert(dl.URL, Equals, "http://different/url")

	// Send fake inspection result to the response channel. The file download handler
	// will open the downloaded file for further processing.
	v.Rch <- nil

	// Wait for the goroutine to finish before deleting spoolDir.
	<-testSync
}

func (t *fileSuite) TestGetSessionIDHeader(c *C) {
	for _, tc := range []struct {
		id    string
		valid bool
	}{
		{"a1391eb7e83a478e88607c373ff00d28", true},
		{"e3d41968-b7bd-11ef-bb53-cb5cd60f3b74", false},
		{"/some/path", false},
		{"/", false},
		{".", false},
		{"", false},
	} {
		r := &http.Request{}
		r.Header = http.Header{"X-Fetch-Session-Id": []string{tc.id}}
		id, err := proxy.GetSessionIDHeader(r)
		if tc.valid {
			c.Assert(err, IsNil)
			c.Check(id, Equals, tc.id)
		} else {
			c.Assert(err, Not(IsNil))
			c.Check(id, Equals, "")
		}
	}
}
