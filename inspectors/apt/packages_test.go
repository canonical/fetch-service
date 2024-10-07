// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2024 Canonical Ltd.
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

package apt_test

import (
	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/canonical/fetch-service/inspectors/apt"
	"github.com/canonical/fetch-service/metadata/digests"
	. "gopkg.in/check.v1"
)

const (
	packagesURL = "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/Packages.xz"
)

// XXX: This file contains minimal testing for apt file formats. Tests
//      will be extended after the metadata format is approved.

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

	/*
		tmp := c.MkDir()
		dest, err := os.Create(filepath.Join(tmp, "0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e.data"))
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

			releaseHash, _ := digests.NewSha256Digest("7a0965cdce7e57af669e786379edcf45953de9bca3763342b870b3ce6d0dd777")
			packagesHash, _ := digests.NewSha256Digest("0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e")
			ctx := metadata.NewInspectionContext()
			ctx.AddReleasePackages(releaseHash, packagesHash, p)

			md := &metadata.Metadata{
				Type:   "application/x-apt-packages",
				Sha256: packagesHash,
				Size:   size,
			}
			di := &metadata.Download{}

			var iface metadata.Inspector
			ins := metadata.AptPackagesInspector{}
			c.Assert(ins, Implements, &iface)

			stop, err := ins.Inspect(filepath.Join(tmp, "0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e.data"), md, di, ctx)
			c.Assert(err, IsNil)
			c.Assert(stop, Equals, true)

			c.Check(md.Name, Equals, "dists/test/Packages.xz")
			c.Check(md.Vendor, Equals, "Acme")
			c.Check(md.Description, Equals, "Apt repository Packages file")
			c.Check(md.Author, Equals, "Acme")
			c.Check(md.Annotations["file.integrity.asserted-by"].Kind, Equals, metadata.Notice)
			c.Check(md.Annotations["file.integrity.asserted-by"].Value, Equals, "7a0965cdce7e57af669e786379edcf45953de9bca3763342b870b3ce6d0dd777")
	*/
}

var packagetests = []struct {
	filename string // test file to read

	sha256       string // package sha256
	pkg          string // package name
	architecture string // package architecture
	version      string // package version
	size         int64  // package size in bytes
}{
	{
		"2048.package",
		"dbe39f124d4f4ee5c440d42805681ba5f64fe23939f460b349735956152361a1",
		"2048",
		"amd64",
		"0.20221023.1237-1",
		14936,
	},
	{
		"python3-grpcio.package",
		"e29b93360482e909d7545335b99fca8aa2f59594d7096a8c99478e9dc7b85631",
		"python3-grpcio",
		"arm64",
		"1.16.1-1ubuntu5",
		747272,
	},
	{
		"btm.package",
		"62b3c95436097e45edeebd72396831938df40de055d2e0dd9fcf276639314799",
		"btm",
		"amd64",
		"0.9.6-4",
		1607224,
	},
}

func (s *aptSuite) TestPackageParsing(c *C) {
	for _, pt := range packagetests {
		filename := filepath.Join("tests", pt.filename)
		reader, err := os.Open(filename)

		c.Assert(err, IsNil)

		entries := map[digests.Sha256Digest]apt.AptPackagesEntry{}

		var num int
		num, err = apt.ParsePackages(reader, entries)

		c.Assert(err, IsNil)
		c.Assert(num, Equals, 1)
		c.Assert(len(entries), Equals, 1)

		for sha256, entry := range entries {
			c.Assert(sha256.String(), Equals, pt.sha256)
			c.Assert(entry.Pkg, Equals, pt.pkg)
			c.Assert(entry.Architecture, Equals, pt.architecture)
			c.Assert(entry.Version, Equals, pt.version)
			c.Assert(entry.Size, Equals, pt.size)
		}
	}
}
