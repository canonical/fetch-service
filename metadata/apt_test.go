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

type aptSuite struct{}

var _ = Suite(&aptSuite{})

const (
	releaseURL  = "http://archive.ubuntu.com/ubuntu/dists/jammy/InRelease"
	packagesURL = "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/Packages.xz"
)

// XXX: This file contains minimal testing for apt file formats. Tests
//      will be extended after the metadata format is approved.

func (s *aptSuite) TestAptReleaseInspector(c *C) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	req, err := http.NewRequest("GET", releaseURL, nil)
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
	md := &metadata.Metadata{Type: "application/x-apt-release", Sha1: h}
	di := &metadata.DownloadInfo{}

	var iface metadata.Inspector
	ins := metadata.AptReleaseInspector{}
	c.Assert(ins, Implements, &iface)

	ctx := metadata.NewInspectionContext()

	stop, err := ins.Inspect(filepath.Join(tmp, "290d07339dde2735121ab03e525ca6593c395a42.bin"), md, di, ctx)
	c.Assert(err, IsNil)
	c.Assert(stop, Equals, true)

	c.Check(md.Name, Equals, "InRelease")
	c.Check(md.Vendor, Equals, "Ubuntu")
	c.Check(md.Description, Equals, "Ubuntu Jammy 22.04")
	c.Check(md.Author, Equals, "Ubuntu")
	c.Check(md.Annotations, HasLen, 0)
}

func (s *aptSuite) TestAptPackagesInspector(c *C) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	req, err := http.NewRequest("GET", packagesURL, nil)
	c.Assert(err, IsNil)

	resp, err := client.Do(req)
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200)

	defer resp.Body.Close()

	tmp := c.MkDir()
	dest, err := os.Create(filepath.Join(tmp, "290d07339dde2735121ab03e525ca6593c395a42.bin"))
	c.Assert(err, IsNil)

	size, err := io.Copy(dest, resp.Body)
	c.Assert(err, IsNil)

	dest.Close()

	// simulate metadata collected from InRelease
	p := metadata.AptReleasePackages{
		Path:   "dists/test/Packages.xz",
		Vendor: "Acme",
		Size:   size,
	}

	h, _ := metadata.NewSha1Digest("290d07339dde2735121ab03e525ca6593c395a42")
	releaseHash, _ := metadata.NewSha1Digest("992b22a7457f7f75b4cfa197393993ebdaa64faf")
	packagesHash, _ := metadata.NewSha256Digest("0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e")
	ctx := metadata.NewInspectionContext()
	ctx.AddReleasePackages(releaseHash, packagesHash, p)

	md := &metadata.Metadata{
		Type:   "application/x-apt-packages",
		Sha1:   h,
		Sha256: packagesHash,
		Size:   size,
	}
	di := &metadata.DownloadInfo{}

	var iface metadata.Inspector
	ins := metadata.AptPackagesInspector{}
	c.Assert(ins, Implements, &iface)

	stop, err := ins.Inspect(filepath.Join(tmp, "290d07339dde2735121ab03e525ca6593c395a42.bin"), md, di, ctx)
	c.Assert(err, IsNil)
	c.Assert(stop, Equals, true)

	c.Check(md.Name, Equals, "dists/test/Packages.xz")
	c.Check(md.Vendor, Equals, "Acme")
	c.Check(md.Description, Equals, "Apt repository Packages file")
	c.Check(md.Author, Equals, "Acme")
	c.Check(md.Annotations["file.integrity.asserted-by"].Kind, Equals, metadata.Notice)
	c.Check(md.Annotations["file.integrity.asserted-by"].Value, Equals, "992b22a7457f7f75b4cfa197393993ebdaa64faf")
}
