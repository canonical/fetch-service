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

package metadata_test

import (
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/metadata"
)

type whlSuite struct{}

var _ = Suite(&whlSuite{})

const (
	testURL = "https://files.pythonhosted.org/packages/1a/27/39933dc51320918ca559eb1abb2ab6d4083f431f1e755c0e79cc717494d7/craft_grammar-1.1.1-py2.py3-none-any.whl"
)

func (s *whlSuite) TestWhlInspector(c *C) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	req, err := http.NewRequest("GET", testURL, nil)
	c.Assert(err, IsNil)

	resp, err := client.Do(req)
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200)

	defer resp.Body.Close()

	tmp := c.MkDir()
	dest, err := os.Create(filepath.Join(tmp, "my-sha1-digest.bin"))
	c.Assert(err, IsNil)
	io.Copy(dest, resp.Body)
	dest.Close()

	md := &metadata.Metadata{Type: "application/x-python-wheel", Sha1: "my-sha1-digest"}
	di := &metadata.DownloadInfo{}

	var iface metadata.Inspector
	ins := metadata.WhlInspector{}
	c.Assert(ins, Implements, &iface)

	stop, err := ins.Inspect(filepath.Join(tmp, "my-sha1-digest.bin"), md, di, nil)
	c.Assert(err, IsNil)
	c.Assert(stop, Equals, true)

	c.Check(md.Name, Equals, "craft-grammar")
	c.Check(md.Vendor, Equals, "Canonical Ltd.")
	c.Check(md.Description, Equals, `"Advance Grammar for Craft Parts"`)
	c.Check(md.Author, Equals, "Canonical Ltd.")
	c.Check(md.AuthorEmail, Equals, "snapcraft@lists.snapcraft.io")
	c.Check(md.License, Equals, "GNU Lesser General Public License v3 (LGPLv3)")
	c.Check(md.Annotations["pip.wheel.version"].Value, Equals, "1.0")
	c.Check(md.Annotations["pip.wheel.metadata.version"].Value, Equals, "2.1")
	c.Check(md.Annotations["pip.wheel.record.check"].Value, Equals, "pass")
}
