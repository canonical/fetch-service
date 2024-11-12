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

func (s *aptSuite) TestAptPackagesInspectorID(c *C) {
	ins := apt.NewAptPackagesInspector(getAptInspectorConfig())
	c.Assert(ins.ID(), Equals, "apt.packages")
}

func (s *aptSuite) TestAptPackagesDetector(c *C) {
	for _, tc := range []struct {
		filename string
		detected bool
	}{
		{"testdata/Packages.xz", true},
		{"testdata/Packages-build-using.xz", true},
		{"testdata/Packages-ppc64el.xz", true},
		{"testdata/InRelease.xz", false},
	} {
		data, err := os.ReadFile(tc.filename)
		c.Assert(err, IsNil)

		res := apt.AptPackagesDetector(data, uint32(len(data)))
		c.Assert(res, Equals, tc.detected, Commentf("test case: %+v", tc))
	}
}

func (s *aptSuite) TestAptPackagesInspectRequest(c *C) {
	for _, tc := range []struct {
		url      string
		detected bool
	}{
		{"http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/by-hash/SHA256/6213291a10046e8188510a0ca41a75daedfb2922940f88888ee815694ab3e7b7", true},
		{"http://archive.ubuntu.com/ubuntu/dists/noble-security/multiverse/binary-ppc64el/by-hash/SHA256/281bdf2a82cbefaa5779127091495e3fb99eccaf05d393d02097896682f198a0", true},
		{"http://some.other.location/Packages.xz", false},
	} {
		ins := apt.NewAptPackagesInspector(getAptInspectorConfig())
		a := metadata.NewArtefact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		if tc.detected {
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

func (s *aptSuite) TestAptPackagesInspectArtefact(c *C) {
	for _, tc := range []struct {
		filename     string
		digest       string
		rejectReason string
	}{
		{"testdata/Packages.xz", "6213291a10046e8188510a0ca41a75daedfb2922940f88888ee815694ab3e7b7", ""},
		{"testdata/Packages-build-using.xz", "f67db265afd9a3a352dcef711099e6ff5eed97ed3ff3f27b90ca5cbc9181ac03", ""},
		{"testdata/Packages-ppc64el.xz", "281bdf2a82cbefaa5779127091495e3fb99eccaf05d393d02097896682f198a0", ""},
		{"testdata/InRelease.xz", "f67db265afd9a3a352dcef711099e6ff5eed97ed3ff3f27b90ca5cbc9181ac03", "error parsing packages file"},
	} {
		ins := apt.NewAptPackagesInspector(getAptInspectorConfig())

		// simulate InRelease entry
		data := apt.NewAptPackages("http://myserver", "jammy", "main", "amd64")
		apt.AptPackagesInspectorAddPackages(ins, "http://myserver", "/path/Packages.xz", data)

		h, _ := digests.NewSha1Digest(tc.digest)
		h2, _ := digests.NewSha256Digest(tc.digest)

		a := metadata.NewArtefact()
		a.CurrentDownload.URL = "http://myserver/path/Packages.xz"
		a.Metadata.Sha1 = h
		a.Metadata.Sha256 = h2
		a.Metadata.Type = "application/x.apt.packages"
		a.MimeType = mimetype.Lookup("application/x.apt.packages")
		a.RequestInspection[ins.ID()] = &Inspection{
			Opinion: opinions.Pending,
			Reason:  "some reason",
		}

		f, err := files.OpenArtefactFile(tc.filename)
		c.Assert(err, IsNil)
		defer f.Close()

		err = ins.InspectArtefact(f, a)
		c.Assert(err, IsNil)

		if tc.rejectReason == "" {
			c.Assert(a.Approved(), Equals, true)
			c.Check(a.Metadata.Type, Equals, "application/x.apt.packages")
			c.Check(a.Metadata.Name, Equals, "Packages.xz")
			c.Check(a.Metadata.Version, Equals, "jammy")
			c.Check(a.Metadata.Description, Equals, "jammy main Packages file")
		} else {
			c.Assert(a.Approved(), Equals, false)
			c.Check(a.ResponseInspection[ins.ID()].Reason, Equals, tc.rejectReason)
		}
	}
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
