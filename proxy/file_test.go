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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/proxy"
)

type fileSuite struct{}

var _ = Suite(&fileSuite{})

func (t *fileSuite) TestNewFileDownloadHandler(c *C) {
	ch := make(chan interface{}, 1)
	body := ioutil.NopCloser(bytes.NewBufferString("Request body"))
	req, err := http.NewRequest("PUT", "http://foo/bar", body)
	req.Header.Set("User-Agent", "test/1.0")
	c.Assert(err, IsNil)

	dir := c.MkDir()

	// download the file
	resp := &http.Response{
		StatusCode: 200,
		Status:     "200 'Tis good",
		Request:    req,
		Body:       ioutil.NopCloser(bytes.NewBufferString("Response body")),
		Header:     http.Header{"Content-Type": []string{"application/x-test"}},
	}

	h, err := proxy.NewFileDownloadHandler(resp, dir, ch)
	c.Assert(err, IsNil)

	data, err := ioutil.ReadAll(h)
	h.Close()
	c.Assert(err, IsNil)
	c.Assert(string(data), Equals, "Response body")

	// check file copy
	fp := filepath.Join(dir, "assets", "176070ca20a7563bed4cef2212a9be37af09f14a", "data.bin")
	content, err := os.ReadFile(fp)
	c.Assert(err, IsNil)
	c.Assert(string(content), Equals, "Response body")

	// check file metadata
	fp = filepath.Join(dir, "assets", "176070ca20a7563bed4cef2212a9be37af09f14a", "metadata.json")
	content, err = os.ReadFile(fp)
	c.Assert(err, IsNil)

	<-ch

	finfo := metadata.FileInfo{}
	err = json.Unmarshal(content, &finfo)
	c.Assert(err, IsNil)
	c.Assert(finfo.Size, Equals, int64(13))
	c.Assert(finfo.Sha1, Equals, "176070ca20a7563bed4cef2212a9be37af09f14a")
	c.Assert(finfo.Sha256, Equals, "f736153d1508e544b6c5ea19e3c2b7448d9af33608d195195e748cb54965e61b")

	// check download metadata
	fp = filepath.Join(dir, "assets", "176070ca20a7563bed4cef2212a9be37af09f14a", "00000000.json")
	content, err = os.ReadFile(fp)
	c.Assert(err, IsNil)

	dinfo := metadata.DownloadInfo{}
	err = json.Unmarshal(content, &dinfo)
	c.Assert(err, IsNil)
	c.Assert(dinfo.StatusCode, Equals, 200)
	c.Assert(dinfo.Status, Equals, "200 'Tis good")
	c.Assert(dinfo.URL, Equals, "http://foo/bar")
	c.Assert(dinfo.ContentType, Equals, "application/x-test")
	c.Assert(dinfo.ResponseHeader["Content-Type"][0], Equals, "application/x-test")

	// download it again
	req, err = http.NewRequest("PUT", "http://different/url", body)
	c.Assert(err, IsNil)

	resp = &http.Response{
		StatusCode: 200,
		Status:     "200 Still good",
		Request:    req,
		Body:       ioutil.NopCloser(bytes.NewBufferString("Response body")), // same content
	}

	h, err = proxy.NewFileDownloadHandler(resp, dir, ch)
	c.Assert(err, IsNil)

	_, err = ioutil.ReadAll(h)
	h.Close()
	c.Assert(err, IsNil)

	<-ch

	// check new download metadata
	fp = filepath.Join(dir, "assets", "176070ca20a7563bed4cef2212a9be37af09f14a", "00000001.json")
	content, err = os.ReadFile(fp)
	c.Assert(err, IsNil)

	dinfo = metadata.DownloadInfo{}
	err = json.Unmarshal(content, &dinfo)
	c.Assert(err, IsNil)
	c.Assert(dinfo.StatusCode, Equals, 200)
	c.Assert(dinfo.Status, Equals, "200 Still good")
	c.Assert(dinfo.URL, Equals, "http://different/url")
}
