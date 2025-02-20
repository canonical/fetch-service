// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2025 Canonical Ltd.
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
	"os"
	"path/filepath"

	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/apt"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
)

func (s *aptSuite) TestPackagesInspectorID(c *C) {
	ins := apt.NewAptPackagesInspector(getAptInspectorConfig())
	c.Assert(ins.ID(), Equals, "apt.packages")
}

type packagesDetectorTest struct {
	filename string // The name of the file to test
	detected bool   // Whether this is expected to be detected
}

var packagesDetectorTests = []packagesDetectorTest{{
	filename: "testdata/Packages.xz",
	detected: true,
}, {
	filename: "testdata/Packages.gz",
	detected: true,
}, {
	filename: "testdata/Packages-build-using.xz",
	detected: true,
}, {
	filename: "testdata/Packages-ppc64el.xz",
	detected: true,
}, {
	filename: "testdata/InRelease.xz",
	detected: false,
}}

func (s *aptSuite) TestPackagesDetector(c *C) {
	for _, tc := range packagesDetectorTests {
		data, err := os.ReadFile(tc.filename)
		c.Assert(err, IsNil)

		res := apt.AptPackagesDetector(data, uint32(len(data)))
		c.Assert(res, Equals, tc.detected, Commentf("test case: %+v", tc))
	}
}

type packagesInspectRequestTest struct {
	url     string // The package request URL
	isValid bool   // Whether this is a valid request URL
}

var packagesInspectRequestTests = []packagesInspectRequestTest{{
	url:     "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/by-hash/SHA256/6213291a10046e8188510a0ca41a75daedfb2922940f88888ee815694ab3e7b7",
	isValid: true,
}, {
	url:     "http://archive.ubuntu.com/ubuntu/dists/noble-security/multiverse/binary-ppc64el/by-hash/SHA256/281bdf2a82cbefaa5779127091495e3fb99eccaf05d393d02097896682f198a0",
	isValid: true,
}, {
	url:     "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/Packages.gz",
	isValid: true,
}, {
	url:     "http://some.other.location/Packages.xz",
	isValid: false,
}}

func (s *aptSuite) TestPackagesInspectRequest(c *C) {
	for _, tc := range packagesInspectRequestTests {
		ins := apt.NewAptPackagesInspector(getAptInspectorConfig())
		a := metadata.NewArtifact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		if tc.isValid {
			insp, ok := a.RequestInspection[ins.ID()]
			c.Assert(ok, Equals, true)
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		} else {
			insp, ok := a.RequestInspection[ins.ID()]
			if ok {
				c.Assert(insp.Opinion, Equals, opinions.Unknown)
			}
		}
	}
}

type packagesInspectArtifactTest struct {
	filename     string // The path to the Packages file
	digest       string // The file's SHA256 digest
	rejectReason string // The reason for rejection, or empty if not rejected
}

var packagesInspectArtifactTests = []packagesInspectArtifactTest{{
	filename:     "testdata/Packages.xz",
	digest:       "6213291a10046e8188510a0ca41a75daedfb2922940f88888ee815694ab3e7b7",
	rejectReason: "",
}, {
	filename:     "testdata/Packages.gz",
	digest:       "646ea3e87bcf00ec9554ba1808d913f40a7403a168de54860143a3206ac2542d",
	rejectReason: "",
}, {
	filename:     "testdata/Packages-build-using.xz",
	digest:       "f67db265afd9a3a352dcef711099e6ff5eed97ed3ff3f27b90ca5cbc9181ac03",
	rejectReason: "",
}, {
	filename:     "testdata/Packages-ppc64el.xz",
	digest:       "281bdf2a82cbefaa5779127091495e3fb99eccaf05d393d02097896682f198a0",
	rejectReason: "",
}, {
	filename:     "testdata/InRelease.xz",
	digest:       "f67db265afd9a3a352dcef711099e6ff5eed97ed3ff3f27b90ca5cbc9181ac03",
	rejectReason: "error parsing packages file",
}}

func (s *aptSuite) TestPackagesInspectArtifact(c *C) {
	for _, tc := range packagesInspectArtifactTests {
		ins := apt.NewAptPackagesInspector(getAptInspectorConfig())

		// simulate InRelease entry
		data := apt.NewAptPackages("http://myserver", "jammy", "main", "amd64")
		apt.AptPackagesInspectorAddPackages(ins, "http://myserver", "/path/Packages.xz", data)

		h2, _ := digests.NewSha256Digest(tc.digest)

		a := metadata.NewArtifact()
		a.CurrentDownload.URL = "http://myserver/path/Packages.xz"
		a.Metadata.Sha256 = h2
		a.Metadata.Type = "application/x.apt.packages"
		a.MimeType = mimetype.Lookup("application/x.apt.packages")
		a.RequestInspection[ins.ID()] = &Inspection{
			Opinion: opinions.Pending,
			Reason:  "some reason",
		}

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer f.Close()

		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		if tc.rejectReason == "" {
			c.Assert(a.Approved(), Equals, true)
			c.Check(a.Metadata.Type, Equals, "application/x.apt.packages")
			c.Check(a.Metadata.Name, Equals, "Packages")
			c.Check(a.Metadata.Version, Equals, "jammy")
			c.Check(a.Metadata.Description, Equals, "jammy main Packages file")
		} else {
			c.Assert(a.Approved(), Equals, false)
			c.Check(a.ResponseInspection[ins.ID()].Reason, Equals, tc.rejectReason)
		}
	}
}

type packageParsingTest struct {
	filename     string // test file to read
	sha256       string // package sha256
	pkg          string // package name
	architecture string // package architecture
	version      string // package version
	size         int64  // package size in bytes
}

var packageParsingTests = []packageParsingTest{{
	filename:     "2048.package",
	sha256:       "dbe39f124d4f4ee5c440d42805681ba5f64fe23939f460b349735956152361a1",
	pkg:          "2048",
	architecture: "amd64",
	version:      "0.20221023.1237-1",
	size:         14936,
}, {
	filename:     "python3-grpcio.package",
	sha256:       "e29b93360482e909d7545335b99fca8aa2f59594d7096a8c99478e9dc7b85631",
	pkg:          "python3-grpcio",
	architecture: "arm64",
	version:      "1.16.1-1ubuntu5",
	size:         747272,
}, {
	filename:     "btm.package",
	sha256:       "62b3c95436097e45edeebd72396831938df40de055d2e0dd9fcf276639314799",
	pkg:          "btm",
	architecture: "amd64",
	version:      "0.9.6-4",
	size:         1607224,
}}

func (s *aptSuite) TestPackageParsing(c *C) {
	for _, pt := range packageParsingTests {
		filename := filepath.Join("testdata", pt.filename)
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
