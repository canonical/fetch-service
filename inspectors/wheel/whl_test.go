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

package wheel_test

import (
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-mmap/mmap"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors"
	"github.com/canonical/fetch-service/inspectors/wheel"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
)

type whlSuite struct{}

func (t *whlSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&whlSuite{})

const (
	whlURL = "https://files.pythonhosted.org/packages/1a/27/39933dc51320918ca559eb1abb2ab6d4083f431f1e755c0e79cc717494d7/craft_grammar-1.1.1-py2.py3-none-any.whl"
)

func (s *whlSuite) TestWhlInspector(c *C) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	req, err := http.NewRequest("GET", whlURL, nil)
	c.Assert(err, IsNil)

	resp, err := client.Do(req)
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200)

	defer resp.Body.Close()

	tmp := c.MkDir()
	dest, err := os.Create(filepath.Join(tmp, "290d07339dde2735121ab03e525ca6593c395a42.bin"))
	c.Assert(err, IsNil)

	_, err = io.Copy(dest, resp.Body)
	c.Assert(err, IsNil)

	dest.Close()

	h, _ := metadata.NewSha1Digest("290d07339dde2735121ab03e525ca6593c395a42")
	a := metadata.NewArtefact(&metadata.DownloadInfo{})
	a.Metadata.Type = "application/x-python-wheel"
	a.Metadata.Sha1 = h

	var iface inspectors.Inspector
	ins := wheel.WhlInspector{}
	c.Assert(ins, Implements, &iface)

	f, err := mmap.Open(filepath.Join(tmp, "290d07339dde2735121ab03e525ca6593c395a42.bin"))
	c.Assert(err, IsNil)
	defer f.Close()

	stop, err := ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)
	c.Assert(stop, Equals, true)

	c.Check(a.Metadata.Name, Equals, "craft-grammar")
	c.Check(a.Metadata.Vendor, Equals, "Canonical Ltd.")
	c.Check(a.Metadata.Description, Equals, `"Advance Grammar for Craft Parts"`)
	c.Check(a.Metadata.Author, Equals, "Canonical Ltd.")
	c.Check(a.Metadata.AuthorEmail, Equals, "snapcraft@lists.snapcraft.io")
	c.Check(a.Metadata.License, Equals, "GNU Lesser General Public License v3 (LGPLv3)")
	c.Check(a.Metadata.Annotations["pip.wheel.version"].Value, Equals, "1.0")
	c.Check(a.Metadata.Annotations["pip.wheel.metadata.version"].Value, Equals, "2.1")
	c.Check(a.Metadata.Annotations["pip.wheel.record.check"].Value, Equals, "pass")
}
