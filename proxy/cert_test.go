// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024 Canonical Ltd.
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
	"os"
	"path/filepath"
	"slices"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/testutils"
)

type certSuite struct{}

func (t *certSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&certSuite{})

func (t *certSuite) TestSetProxyCA(c *C) {
	ca, err := proxy.CreateProxyCA(testutils.ProxyCert, testutils.ProxyKey)
	c.Assert(err, IsNil)

	err = proxy.SetProxyCA(ca)
	c.Assert(err, IsNil)
}

func (t *certSuite) TestLoadCertificate(c *C) {
	dir := c.MkDir()
	certPath := filepath.Join(dir, "cert.txt")
	keyPath := filepath.Join(dir, "key.txt")

	err := os.WriteFile(certPath, []byte("cert data"), 0644)
	c.Assert(err, IsNil)

	err = os.WriteFile(keyPath, []byte("key data"), 0644)
	c.Assert(err, IsNil)

	for _, tc := range []struct {
		cert   string
		key    string
		errMsg string
	}{
		{certPath, keyPath, ""},
		{certPath, "", "HTTPS proxy key path not specified"},
		{"", "", "HTTPS proxy certificate path not specified"},
		{"/other/path", keyPath, "open /other/path: no such file or directory"},
		{certPath, "/other/path", "open /other/path: no such file or directory"},
	} {
		certData, keyData, err := proxy.LoadCertificate(tc.cert, tc.key)
		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Check(string(certData), Equals, "cert data")
			c.Check(string(keyData), Equals, "key data")
		} else {
			c.Assert(err, ErrorMatches, tc.errMsg)
		}
	}
}

func (t *certSuite) TestUpdateCert(c *C) {
	for _, tc := range []struct {
		dryRun  bool
		payload []byte
		errMsg  string
	}{
		{true, append(append(testutils.ProxyCert, []byte("\n\n")...), testutils.ProxyKey...), ""},
		{false, append(append(testutils.ProxyCert, []byte("\n\n")...), testutils.ProxyKey...), ""},
		{true, []byte{}, "cannot parse certificate and key"},
		{true, testutils.ProxyCert, "cannot parse certificate and key"},
		{true, testutils.ProxyCert, "cannot parse certificate and key"},
	} {
		dir := c.MkDir()
		certPath := filepath.Join(dir, "cert.txt")
		keyPath := filepath.Join(dir, "key.txt")
		err := proxy.UpdateCert(tc.dryRun, tc.payload, certPath, keyPath)

		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			if tc.dryRun {
				_, err := os.Stat(certPath)
				c.Assert(err != nil && os.IsNotExist(err), Equals, true)

				_, err = os.ReadFile(keyPath)
				c.Assert(err != nil && os.IsNotExist(err), Equals, true)
			} else {
				cert, err := os.ReadFile(certPath)
				c.Assert(err, IsNil)
				c.Check(slices.Equal(cert, testutils.ProxyCert), Equals, true)

				key, err := os.ReadFile(keyPath)
				c.Assert(err, IsNil)
				c.Check(slices.Equal(key, testutils.ProxyKey), Equals, true)
			}
		} else {
			c.Assert(err, ErrorMatches, tc.errMsg)
		}
	}
}

func (t *certSuite) TestUpdateCertFiles(c *C) {
	dir := c.MkDir()
	certPath := filepath.Join(dir, "cert.txt")
	keyPath := filepath.Join(dir, "key.txt")
	err := proxy.UpdateCertFiles(certPath, keyPath, testutils.ProxyCert, testutils.ProxyKey)
	c.Assert(err, IsNil)

	certData, err := os.ReadFile(certPath)
	c.Assert(err, IsNil)
	c.Check(slices.Compare(certData, testutils.ProxyCert), Equals, 0)

	keyData, err := os.ReadFile(keyPath)
	c.Assert(err, IsNil)
	c.Check(slices.Compare(keyData, testutils.ProxyKey), Equals, 0)
}

func (t *certSuite) TestSplitCertKey(c *C) {
	for _, tc := range []struct {
		data   []byte
		cert   []byte
		key    []byte
		errMsg string
	}{
		{[]byte("block1\nblock1\n\nblock2\nblock2"), []byte("block1\nblock1"), []byte("block2\nblock2"), ""},
		{[]byte("block1\nblock1"), []byte{}, []byte{}, "cannot parse certificate and key"},
		{[]byte("block1\n\nblock2\n\nblock3"), []byte("block1"), []byte("block2\n\nblock3"), ""},
	} {
		cert, key, err := proxy.SplitCertKey(tc.data)
		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Check(slices.Compare(cert, tc.cert), Equals, 0)
			c.Check(slices.Compare(key, tc.key), Equals, 0)
		} else {
			c.Assert(err, ErrorMatches, tc.errMsg)
		}
	}
}
