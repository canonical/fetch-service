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

package deb_test

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
	"github.com/canonical/fetch-service/inspectors/deb"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
)

type debSuite struct{}

func (t *debSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&debSuite{})

const (
	debURL = "http://launchpadlibrarian.net/592566337/hello_2.10-2ubuntu4_amd64.deb"
)

func (s *debSuite) TestDebInspector(c *C) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	req, err := http.NewRequest("GET", debURL, nil)
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
	md := &metadata.Metadata{Type: "application/vnd.debian.binary-package", Sha1: h}
	di := &metadata.DownloadInfo{}

	var iface inspectors.Inspector
	ins := deb.DebInspector{}
	c.Assert(ins, Implements, &iface)

	// TODO: inject Packages.xz data into inspection context to validade the deb file

	f, err := mmap.Open(filepath.Join(tmp, "290d07339dde2735121ab03e525ca6593c395a42.bin"))
	c.Assert(err, IsNil)
	defer f.Close()

	stop, err := ins.InspectArtefact(f, md, di)
	c.Assert(err, IsNil)
	c.Assert(stop, Equals, true)

	c.Check(md.Name, Equals, "hello")
	c.Check(md.Version, Equals, "2.10-2ubuntu4")
	c.Check(md.Vendor, Equals, "Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>")
	c.Check(md.Description, Equals, "Example package based on GNU hello")
	c.Check(md.Author, Equals, "") // FIXME: deb inspector needs a better author email parser
	c.Check(md.AuthorEmail, Equals, "Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>")
	c.Check(md.License, Equals, "") // FIXME: copyright file is not in machine-readable format
	c.Check(md.Annotations["deb.debian-binary.version"].Value, Equals, "2.0")
}
