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

package deb_test

import (
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

type debSuite struct{}

func (t *debSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&debSuite{})

func Test(t *testing.T) { TestingT(t) }

func getTestAptConfig() config.AptInspectorConfig {
	return config.AptInspectorConfig{
		Repositories: map[string]config.AptInspectorConfigRepository{
			"default": {
				Urls:       []glob.Glob{glob.MustCompile("http://archive.ubuntu.com/ubuntu")},
				Dists:      []glob.Glob{glob.MustCompile("jammy")},
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

func (s *debSuite) TestInspectRequest(c *C) {
	for _, tc := range []struct {
		url     string
		pending bool
	}{
		{"http://archive.ubuntu.com/ubuntu/pool/main/libe/liberror-perl/liberror-perl_0.17029-1_all.deb", true},
		{"http://archive.ubuntu.com/ubuntu/pool/main/b/borgmatic/borgmatic_1.7.9-0ubuntu1~bpo22.04.1_all.deb", true},
		{"http://archive.ubuntu.com/ubuntu/pool/universe/b/borgmatic/borgmatic_1.7.9-0ubuntu1~bpo22.04.1_all.deb", true},
		{"http://not-archive.ubuntu.com/ubuntu/pool/main/libe/liberror-perl/liberror-perl_0.17029-1_all.deb", false},
	} {
		ins := deb.NewDebInspector(getTestAptConfig())
		a := metadata.NewArtefact()
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

func (s *debSuite) TestDebArtefactInspector(c *C) {
	for _, tc := range []struct {
		testfile string
		approved bool
	}{
		{"testdata/hello_2.10-2ubuntu4_amd64.deb", true},
		{"testdata/2048.package", false},
	} {
		a := metadata.NewArtefact()
		a.Metadata.Type = "application/vnd.debian.binary-package"
		a.MimeType = mimetype.Lookup("application/vnd.debian.binary-package")

		f, err := files.OpenArtefactFile(tc.testfile)
		c.Assert(err, IsNil)
		defer f.Close()

		ins := deb.NewDebInspector(getTestAptConfig())
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtefact(f, a)
		c.Assert(err, IsNil)
		c.Assert(a.Approved(), Equals, tc.approved)

		if tc.approved {
			c.Check(a.Metadata.Name, Equals, "hello")
			c.Check(a.Metadata.Version, Equals, "2.10-2ubuntu4")
			c.Check(a.Metadata.Vendor, Equals, "Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>")
			c.Check(a.Metadata.Description, Equals, "Example package based on GNU hello")
			c.Check(a.Metadata.Author, Equals, "") // FIXME: deb inspector needs a better author email parser
			c.Check(a.Metadata.AuthorEmail, Equals, "Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>")
			c.Check(a.Metadata.License, Equals, "GFDL-1.3-or-later and/or GPL-3.0-or-later") // this is what licensecheck says
			c.Check(a.Metadata.Architecture, Equals, "amd64")
		}
	}
}
