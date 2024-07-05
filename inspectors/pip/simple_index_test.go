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

package pip_test

import (
	"os"
	"path/filepath"

	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/pip"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type simpleIndexSuite struct{}

func (t *simpleIndexSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&simpleIndexSuite{})

func (s *simpleIndexSuite) TestSimpleIndexInspectorInterface(c *C) {
	var iface Inspector
	ins := pip.NewSimpleIndexInspector()
	c.Assert(ins, Implements, &iface)

}

func (s *simpleIndexSuite) TestSimpleIndexInspectorID(c *C) {
	ins := pip.NewSimpleIndexInspector()
	c.Assert(ins.ID(), Equals, "pip.simple-index")
}

func (s *simpleIndexSuite) TestInspectRequest(c *C) {
	for _, tc := range []struct {
		url      string
		name     string
		approved bool
	}{
		{"https://pypi.org:443/simple/foo/", "foo", true},
		{"https://pypi.org:443/simple/foo-bar/", "foo-bar", true},
		{"https://pypi.org:443/simple/foo", "", false},
		{"http://pypi.org/simple/foo", "", false},
		{"https://pypi.org:443/simple/foo/bar", "", false},
		{"https://pypi.org:444/simple/foo", "", false},
		{"https://example.com/simple/foo", "", false},
		{"https://pypi.org:443/simple", "", false},
		{"ahttps://pypi.org:443/simple/foo", "", false},
	} {
		ins := pip.NewSimpleIndexInspector()
		a := metadata.NewArtefact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, tc.approved)
		if tc.approved {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		}

		if tc.name != "" {
			c.Assert(a.RequestInspection["pip.simple-index"].Annotations["package-name"], Equals, tc.name)
		}
	}
}

func (s *simpleIndexSuite) TestInspectArtefactBadType(c *C) {
	ins := pip.NewSimpleIndexInspector()
	a := metadata.NewArtefact()
	a.Metadata.Type = "text/plain"
	a.MimeType = mimetype.Lookup("text/plain")
	a.CurrentDownload.URL = "https://pypi.org:443/simple/foo/"

	err := ins.InspectArtefact(nil, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
	c.Assert(a.Rejected(), Equals, true)
}

func (s *simpleIndexSuite) TestWheelInspectArtefactBadContent(c *C) {
	tmp := c.MkDir()
	filename := filepath.Join(tmp, "index.html")
	err := os.WriteFile(filename, []byte("random content"), 0755)
	c.Assert(err, IsNil)

	ins := pip.NewSimpleIndexInspector()
	a := metadata.NewArtefact()
	a.Metadata.Type = "text/plain"
	a.MimeType = mimetype.Lookup("text/plain")
	a.CurrentDownload.URL = "https://pypi.org:443/simple/foo/"
	a.SetRequestPending(ins, "test")

	f, err := inspectors.OpenArtefactFile(filename)
	c.Assert(err, IsNil)
	defer f.Close()

	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
	c.Assert(a.Rejected(), Equals, true)
	c.Assert(a.ResponseInspection, DeepEquals, metadata.InspectionMap{})
}

func (s *simpleIndexSuite) TestWheelInspectArtefact(c *C) {
	tmp := c.MkDir()
	filename := filepath.Join(tmp, "index.html")
	err := os.WriteFile(filename, []byte("<!DOCTYPE html>\n"+
		"<html>\n"+
		"  <head>\n"+
		`    <meta name="pypi:repository-version" content="1.1">\n`+
		"    <title>Links for foobar</title>\n"+
		"  </head>\n"+
		"  <body>\n"+
		"    <h1>Links for foobar</h1>\n"+
		"  </body>\n"+
		"</html>"), 0755)
	c.Assert(err, IsNil)

	ins := pip.NewSimpleIndexInspector()
	h, _ := digests.NewSha1Digest("85fc2d2a3764089191e57cd552601278a5985c46")

	a := metadata.NewArtefact()
	a.Metadata.Type = "text/html"
	a.Metadata.Sha1 = h
	a.MimeType = mimetype.Lookup("text/html")
	a.CurrentDownload.URL = "https://pypi.org:443/simple/foobar/"
	a.RequestInspection["pip.simple-index"] = &Inspection{
		Opinion:     opinions.Pending,
		Reason:      "some reason",
		Annotations: Annotation{"package-name": "foobar"},
	}

	f, err := inspectors.OpenArtefactFile(filename)
	c.Assert(err, IsNil)
	defer f.Close()

	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, true)

	c.Check(a.Metadata.Type, Equals, "text/html")
	c.Check(a.Metadata.Name, Equals, "Simple index for 'foobar'")
	c.Check(a.Metadata.Vendor, Equals, "pypi.org")
	c.Check(a.Metadata.Description, Equals, "PyPI repository index HTML file for package 'foobar'")
	c.Check(a.Metadata.Author, Equals, "pypi.org")
	c.Check(a.Metadata.AuthorEmail, Equals, "")
	c.Check(a.Metadata.License, Equals, "")
	c.Assert(a.Approved(), Equals, true)
}
