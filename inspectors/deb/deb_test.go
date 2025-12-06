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

package deb_test

import (
	"os"
	"strings"
	"testing"

	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/inspectors/apt/config"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/deb"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type debSuite struct {
	slog logger.Logger
}

func (t *debSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&debSuite{logger.NewSessionLogger("test")})

func Test(t *testing.T) { TestingT(t) }

func getTestAptConfig() config.AptInspectorConfig {
	return config.AptInspectorConfig{
		Repositories: map[string]config.AptInspectorConfigRepository{
			"default": {
				URLs:       []glob.Glob{glob.MustCompile("http://archive.ubuntu.com/ubuntu")},
				Suites:     []glob.Glob{glob.MustCompile("jammy")},
				Components: []glob.Glob{glob.MustCompile("main")},
				PublicKey:  "",
			},
		},
	}
}

func (s *debSuite) TestDebInspectorID(c *C) {
	ins := deb.NewDebInspector(getTestAptConfig())
	c.Assert(ins.ID(), Equals, "deb")
}

func (s *debSuite) TestDebInspectorInterface(c *C) {
	var iface Inspector
	ins := deb.NewDebInspector(config.AptInspectorConfig{})
	c.Assert(ins, Implements, &iface)

}

type debInspectRequestTest struct {
	url     string // The request URL
	pending bool   // Whether this should be set to pending state
}

var debInspectRequestTests = []debInspectRequestTest{{
	url:     "http://archive.ubuntu.com/ubuntu/pool/main/libe/liberror-perl/liberror-perl_0.17029-1_all.deb",
	pending: true,
}, {
	url:     "http://archive.ubuntu.com/ubuntu/pool/main/b/borgmatic/borgmatic_1.7.9-0ubuntu1~bpo22.04.1_all.deb",
	pending: true,
}, {
	url:     "http://archive.ubuntu.com/ubuntu/pool/universe/b/borgmatic/borgmatic_1.7.9-0ubuntu1~bpo22.04.1_all.deb",
	pending: true,
}, {
	url:     "http://not-archive.ubuntu.com/ubuntu/pool/main/libe/liberror-perl/liberror-perl_0.17029-1_all.deb",
	pending: false,
}}

func (s *debSuite) TestDebInspectRequest(c *C) {
	for _, tc := range debInspectRequestTests {
		ins := deb.NewDebInspector(getTestAptConfig())
		a := metadata.NewArtifact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, tc.pending, Commentf("test case: %+v", tc))
		if tc.pending {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		}
	}
}

type debArtifactInspectorTest struct {
	filename      string // The deb file to be inspected
	debInPackages bool   // Deb file listed in Packages file
	validPackages bool   // Packages file is validated against Releases
	approved      bool   // Whether it should be approved
	reason        string // The reasoning for the inspection opinion
}

var debArtifactInspectorTests = []debArtifactInspectorTest{{
	filename:      "testdata/hello_2.10-2ubuntu4_amd64.deb",
	debInPackages: true,
	validPackages: true,
	approved:      true,
	reason:        "deb package successfully parsed and listed in valid Packages file",
}, {
	filename:      "testdata/hello_2.10-2ubuntu4_amd64.deb",
	debInPackages: true,
	validPackages: false,
	approved:      false,
	reason:        "deb file listed in invalid Packages file",
}, {
	filename:      "testdata/hello_2.10-2ubuntu4_amd64.deb",
	debInPackages: false,
	validPackages: true,
	approved:      false,
	reason:        "deb file not verified against Packages file",
}, {
	filename:      "testdata/2048.package",
	debInPackages: true,
	validPackages: true,
	approved:      false,
	reason:        "ar parse error: unexpected EOF",
}, {
	filename:      "testdata/2048.package",
	debInPackages: false,
	validPackages: true,
	approved:      false,
	reason:        "ar parse error: unexpected EOF",
}}

func (s *debSuite) TestDebArtifactInspector(c *C) {
	for _, tc := range debArtifactInspectorTests {
		a := metadata.NewArtifact()
		a.Metadata.Type = "application/vnd.debian.binary-package"
		a.MimeType = mimetype.Lookup("application/vnd.debian.binary-package")

		if tc.debInPackages {
			a.ResponseInspection["apt.packages"] = &Inspection{
				Opinion: opinions.Unknown,
				Reason:  "some reason",
				Annotations: Annotation{
					"packages-file":     "d29cb24c93fec5f43255706bce7eb46d4779952039d8c68ac1bb14a6f3655ce2",
					"packages-is-valid": tc.validPackages,
				},
			}
		}

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		ins := deb.NewDebInspector(getTestAptConfig())
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Assert(a.Approved(), Equals, tc.approved, Commentf("test case: %+v", tc))
		c.Check(a.ResponseInspection["deb"].Reason, Equals, tc.reason)

		if tc.approved {
			c.Check(a.Metadata.Name, Equals, "hello")
			c.Check(a.Metadata.Version, Equals, "2.10-2ubuntu4")
			c.Check(a.Metadata.Vendor, Equals, "Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>")
			c.Check(a.Metadata.Description, Equals, "Example package based on GNU hello")
			c.Check(a.Metadata.Author, Equals, "")
			c.Check(a.Metadata.AuthorEmail, Equals, "")
			c.Check(a.Metadata.License, Equals, "GFDL-1.3-or-later and/or GPL-3.0-or-later") // this is what licensecheck says
			c.Check(a.Metadata.Architecture, Equals, "amd64")
		}
	}
}

func (s *debSuite) TestParseControl(c *C) {
	reader, err := os.OpenFile("testdata/libcurl-gnutls.control", os.O_RDONLY, 0)
	c.Assert(err, IsNil)

	meta := ArtifactMetadata{}

	err = deb.ParseControl(reader, &meta)
	c.Assert(err, IsNil)

	c.Check(meta.Name, Equals, "libcurl3-gnutls")
	c.Check(meta.Version, Equals, "7.81.0-1ubuntu1.19")
	c.Check(meta.Vendor, Equals, "Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>")
	c.Check(meta.Description, Equals, "Easy-to-use client-side URL transfer library (GnuTLS flavour)")
	c.Check(meta.Author, Equals, "")
	c.Check(meta.AuthorEmail, Equals, "")
	c.Check(meta.License, Equals, "")
	c.Check(meta.Architecture, Equals, "amd64")
	c.Check(meta.SourcePackage, Equals, "curl")
}

type debBinaryVersionTest struct {
	content         string // debian-binary file content
	expectedVersion string // Expected version
}

var debBinaryVersionTests = []debBinaryVersionTest{{
	content:         "",
	expectedVersion: "",
}, {
	content:         "2.0",
	expectedVersion: "2.0",
}, {
	content:         "2.0\n",
	expectedVersion: "2.0",
}, {
	content:         "2.0\nxyz",
	expectedVersion: "2.0",
}}

func (s *debSuite) TestDebBinaryVersion(c *C) {
	for _, tc := range debBinaryVersionTests {
		r := strings.NewReader(tc.content)
		ins := deb.NewDebInspector(getTestAptConfig())
		res := deb.DebInspectorGetDebianBinaryVersion(ins, r)
		c.Check(res, Equals, tc.expectedVersion)
	}

}

type readDebMetadataTest struct {
	filename string // The name of the deb file to test
	name     string // The expected package name
	version  string // The expected package version
	errMsg   string // The expected error string, if not empty
}

var readDebMetadataTests = []readDebMetadataTest{{
	filename: "testdata/hello_2.10-1_amd64.deb", // deb has gzipped control and xz data
	name:     "hello",
	version:  "2.10-1",
	errMsg:   "",
}, {
	filename: "testdata/hello_2.10-2_amd64.deb", // deb has xz control and gzipped data
	name:     "hello",
	version:  "2.10-1", // it's 1 because the deb file was renamed but contents not changed
	errMsg:   "",
}, {
	filename: "testdata/hello_2.10-2ubuntu4_amd64.deb", // deb has zstd control and data
	name:     "hello",
	version:  "2.10-2ubuntu4",
	errMsg:   "",
}, {
	filename: "testdata/2048.package", // not a deb file
	name:     "",
	version:  "",
	errMsg:   "ar parse error: unexpected EOF",
}, {
	filename: "testdata/hello_2.10-3_amd64.deb", // deb is missing the control file
	name:     "",
	version:  "",
	errMsg:   "cannot read name and version from control metadata",
}}

func (s *debSuite) TestReadDebMetadata(c *C) {
	for _, tc := range readDebMetadataTests {
		r, err := os.Open(tc.filename)
		c.Assert(err, IsNil)

		am := ArtifactMetadata{}
		ins := deb.NewDebInspector(getTestAptConfig())
		err = deb.DebInspectorReadDebMetadata(ins, r, &am, s.slog)
		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Check(am.Name, Equals, tc.name)
			c.Check(am.Version, Equals, tc.version)
			c.Check(am.License, Equals, "GFDL-1.3-or-later and/or GPL-3.0-or-later")
		} else {
			c.Assert(err, ErrorMatches, tc.errMsg)
		}
	}

}
