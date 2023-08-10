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
	"strings"
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
	dest, err := os.Create(filepath.Join(tmp, "my-sha1-digest.bin"))
	c.Assert(err, IsNil)

	_, err = io.Copy(dest, resp.Body)
	c.Assert(err, IsNil)

	dest.Close()

	md := &metadata.Metadata{Type: "application/x-apt-release", Sha1: "my-sha1-digest"}
	di := &metadata.DownloadInfo{}

	var iface metadata.Inspector
	ins := metadata.AptReleaseInspector{}
	c.Assert(ins, Implements, &iface)

	ctx := metadata.NewInspectionContext()

	stop, err := ins.Inspect(filepath.Join(tmp, "my-sha1-digest.bin"), md, di, ctx)
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
	dest, err := os.Create(filepath.Join(tmp, "my-sha1-digest.bin"))
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

	ctx := metadata.NewInspectionContext()
	ctx.AddReleasePackages("release-digest", "my-sha256-digest", p)

	md := &metadata.Metadata{
		Type:   "application/x-apt-packages",
		Sha1:   "my-sha1-digest",
		Sha256: "my-sha256-digest",
		Size:   size,
	}
	di := &metadata.DownloadInfo{}

	var iface metadata.Inspector
	ins := metadata.AptPackagesInspector{}
	c.Assert(ins, Implements, &iface)

	stop, err := ins.Inspect(filepath.Join(tmp, "my-sha1-digest.bin"), md, di, ctx)
	c.Assert(err, IsNil)
	c.Assert(stop, Equals, true)

	c.Check(md.Name, Equals, "dists/test/Packages.xz")
	c.Check(md.Vendor, Equals, "Acme")
	c.Check(md.Description, Equals, "Apt repository Packages file")
	c.Check(md.Author, Equals, "Acme")
	c.Check(md.Annotations["file.integrity.asserted-by"].Kind, Equals, metadata.Notice)
	c.Check(md.Annotations["file.integrity.asserted-by"].Value, Equals, "release-digest")
}

func (s *aptSuite) TestAptLegacyReleaseDetector(c *C) {
	entries := []string{"Component: a", "Version: b", "Label: c", "Architecture: d", "Origin: e", "Archive: f"}

	// Ensure all entries are necessary
	e := make([]string, len(entries))
	for i := range entries {
		copy(e, entries)
		e[i] = e[len(e)-1] // replace with last element
		content := []byte(strings.Join(e[:len(e)-1], "\n"))
		c.Check(metadata.AptLegacyReleaseDetector(content, 256), Equals, false)
	}

	content := []byte(strings.Join(entries, "\n"))
	c.Check(metadata.AptLegacyReleaseDetector(content, 256), Equals, true)

	// Extra entries not allowed
	extra := append([]string{"Extra: z"}, entries...)
	content = []byte(strings.Join(extra, "\n"))
	c.Check(metadata.AptLegacyReleaseDetector(content, 256), Equals, false)
}
