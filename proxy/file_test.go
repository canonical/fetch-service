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
	"io/ioutil"
	"net/http"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/proxy"
)

type fileSuite struct{}

var _ = Suite(&fileSuite{})

func (t *fileSuite) TestNewFileDownloadHandler(c *C) {
	ch := make(chan interface{}, 1)
	body := ioutil.NopCloser(bytes.NewBufferString("Request body"))
	req, err := http.NewRequest("GET", "http://foo/bar", body)
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

	go func(body io.ReadCloser) {
		data, err := ioutil.ReadAll(body)
		body.Close()
		c.Assert(err, IsNil)
		c.Assert(string(data), Equals, "Response body")
	}(h)

	msg := <-ch
	v := msg.(metadata.FileDownload)

	mver := fmt.Sprintf("%d.%d", metadata.MetadataVersionMajor, metadata.MetadataVersionMinor)

	// check file metadata
	c.Assert(v.Md.MetadataVersion, Equals, mver)
	c.Assert(v.Md.Sha1.String(), Equals, "176070ca20a7563bed4cef2212a9be37af09f14a")
	c.Assert(v.Md.Sha256.String(), Equals, "f736153d1508e544b6c5ea19e3c2b7448d9af33608d195195e748cb54965e61b")
	c.Assert(v.Md.Size, Equals, int64(13))

	// check download info
	c.Assert(v.Info.StatusCode, Equals, 200)
	c.Assert(v.Info.Status, Equals, "200 'Tis good")
	c.Assert(v.Info.Method, Equals, "GET")
	c.Assert(v.Info.URL, Equals, "http://foo/bar")
	c.Assert(v.Info.ContentType, Equals, "application/x-test")
	c.Assert(v.Info.ResponseHeader["Content-Type"][0], Equals, "application/x-test")
	c.Assert(v.Info.Sha1.String(), Equals, "176070ca20a7563bed4cef2212a9be37af09f14a")

	v.Rch <- nil

	// download it again
	req, err = http.NewRequest("POST", "http://different/url", body)
	c.Assert(err, IsNil)

	resp = &http.Response{
		StatusCode: 200,
		Status:     "200 Still good",
		Request:    req,
		Body:       ioutil.NopCloser(bytes.NewBufferString("Response body")), // same content
	}

	h, err = proxy.NewFileDownloadHandler(resp, dir, ch)
	c.Assert(err, IsNil)

	go func(body io.ReadCloser) {
		_, err = ioutil.ReadAll(body)
		body.Close()
		c.Assert(err, IsNil)
	}(h)

	msg = <-ch
	v = msg.(metadata.FileDownload)

	// check file metadata
	c.Assert(v.Md.Sha1.String(), Equals, "176070ca20a7563bed4cef2212a9be37af09f14a")

	// check download info
	c.Assert(v.Info.StatusCode, Equals, 200)
	c.Assert(v.Info.Status, Equals, "200 Still good")
	c.Assert(v.Info.Method, Equals, "POST")
	c.Assert(v.Info.URL, Equals, "http://different/url")

	v.Rch <- nil
}
