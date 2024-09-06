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

	certData, keyData, err := proxy.LoadCertificate(certPath, keyPath)
	c.Assert(err, IsNil)
	c.Check(string(certData), Equals, "cert data")
	c.Check(string(keyData), Equals, "key data")
}
