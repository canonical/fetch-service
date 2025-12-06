// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024 Canonical Ltd.
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
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"

	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/pip"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type sdistSuite struct{}

func (t *sdistSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&sdistSuite{})

func (s *sdistSuite) TestSdistInspectorInterface(c *C) {
	var iface Inspector
	ins := pip.NewSdistInspector()
	c.Assert(ins, Implements, &iface)

}

func (s *sdistSuite) TestSdistInspectorID(c *C) {
	ins := pip.NewSdistInspector()
	c.Assert(ins.ID(), Equals, "pip.sdist")

}

func (s *sdistSuite) TestInspectRequest(c *C) {
	for _, tc := range []struct {
		url      string
		approved bool
	}{
		{"https://files.pythonhosted.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.tar.gz", true},
		{"http://files.pythonhosted.org/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.tar.gz", false},
		{"https://files.pythonhosted.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789a/foobar-1.0.0.tar.gz", false},
		{"https://files.pythonhosted.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.whl", false},
		{"https://files.pythonhosted.org:443/packages/0f9a0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.tar.gz", false},
		{"https://pypi.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.tar.gz", false},
		{"ahttps://files.pythonhosted.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.tar.gz", false},
	} {
		ins := pip.NewSdistInspector()
		a := metadata.NewArtifact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, tc.approved)
		if tc.approved {
			c.Assert(insp.Opinion, Equals, opinions.Unknown)
			c.Assert(insp.Reason, Equals, "unsupported origin")
		}
	}
}

func (s *sdistSuite) TestInspectArtifactBadType(c *C) {
	ins := pip.NewSdistInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/zip")

	err := ins.InspectArtifact(nil, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
	c.Assert(a.Rejected(), Equals, true)
}

func (s *sdistSuite) TestSdistInspectArtifactReadMetadata(c *C) {
	tmp := c.MkDir()
	testfile := filepath.Join(tmp, "test.tar.gz")

	pkgInfoContent := "Metadata-Version: 2.1\n" +
		"Name: craft-parts\n" +
		"Version: 1.31.0\n" +
		"Summary: Craft parts tooling\n" +
		"Home-page: https://github.com/canonical/craft-parts\n" +
		"Author: Canonical Ltd.\n" +
		"Author-email: snapcraft@lists.snapcraft.io\n" +
		"License: GNU General Public License v3\n" +
		"Classifier: Development Status :: 4 - Beta\n" +
		"Classifier: Intended Audience :: Developers\n" +
		"Classifier: License :: OSI Approved :: GNU Lesser General Public License v3 (LGPLv3)\n" +
		"Classifier: Natural Language :: English\n" +
		"Requires-Dist: overrides!=7.6.0\n" +
		"Requires-Dist: PyYAML\n" +
		"Requires-Dist: pydantic<2.0.0,>=1.9.0\n" +
		"Requires-Dist: pydantic-yaml[pyyaml]<1.0.0,>=0.11.0\n" +
		"Provides-Extra: apt\n" +
		"Requires-Dist: python-apt>=2.0.0; extra == \"apt\"\n" +
		"\n" +
		"# Craft Parts\n" +
		"\n" +
		"Craft-parts provides a mechanism to obtain data from different sources,\n" +
		"process it in various ways, and prepare a filesystem subtree suitable for\n" +
		"deployment.\n"

	// Create test sdist
	sf, err := os.Create(testfile)
	c.Assert(err, IsNil)
	defer func() { _ = sf.Close() }()

	zf := gzip.NewWriter(sf)
	tf := tar.NewWriter(zf)

	hdr := &tar.Header{
		Name: "foobar-1.0/",
		Mode: 0755,
	}
	err = tf.WriteHeader(hdr)
	c.Assert(err, IsNil)

	hdr = &tar.Header{
		Name: "foobar-1.0/PKG-INFO",
		Mode: 0644,
		Size: int64(len(pkgInfoContent)),
	}
	err = tf.WriteHeader(hdr)
	c.Assert(err, IsNil)

	_, err = tf.Write([]byte(pkgInfoContent))
	c.Assert(err, IsNil)

	err = tf.Close()
	c.Assert(err, IsNil)

	err = zf.Close()
	c.Assert(err, IsNil)

	// Inspect test sdist
	f, err := files.OpenArtifactFile(testfile)
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	ins := pip.NewSdistInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/gzip")
	a.SetRequestPending(ins, "test")

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, true)
	c.Assert(a.Metadata.Name, Equals, "craft-parts")
	c.Assert(a.Metadata.Version, Equals, "1.31.0")
	c.Assert(a.Metadata.Description, Equals, "Craft parts tooling")
	c.Assert(a.Metadata.Vendor, Equals, "Canonical Ltd.")
	c.Assert(a.Metadata.Author, Equals, "Canonical Ltd.")
	c.Assert(a.Metadata.AuthorEmail, Equals, "snapcraft@lists.snapcraft.io")
	c.Assert(a.Metadata.License, Equals, "GPL-3 and/or LGPL-3")
	c.Assert(a.ResponseInspection["pip.sdist"], DeepEquals, &Inspection{
		Opinion: opinions.Approved,
		Reason:  "sdist file successfully parsed",
		Annotations: Annotation{
			"metadata-version": "2.1",
		},
	})
}

func (s *sdistSuite) TestSdistInspectArtifactBadFormat(c *C) {
	tmp := c.MkDir()
	testfile := filepath.Join(tmp, "test.tar.gz")

	for _, tc := range []struct {
		content  string
		approved bool
	}{
		{"something else", false},
		{"metadata-version: 2.1\nname: test\n", false},
		{"name: test\nversion: 1.0\n", false},
		{"metadata-version: 2.1\nversion: 1.0\n", false},
		{"metadata-version: 2.1\nname: test\nversion: 1.0\n", true},
	} {

		// Create test sdist
		sf, err := os.Create(testfile)
		c.Assert(err, IsNil)
		defer func() { _ = sf.Close() }()

		zf := gzip.NewWriter(sf)
		tf := tar.NewWriter(zf)

		hdr := &tar.Header{
			Name: "foobar-1.0/",
			Mode: 0755,
		}
		err = tf.WriteHeader(hdr)
		c.Assert(err, IsNil)

		hdr = &tar.Header{
			Name: "foobar-1.0/PKG-INFO",
			Mode: 0644,
			Size: int64(len(tc.content)),
		}
		err = tf.WriteHeader(hdr)
		c.Assert(err, IsNil)

		_, err = tf.Write([]byte(tc.content))
		c.Assert(err, IsNil)

		err = tf.Close()
		c.Assert(err, IsNil)

		err = zf.Close()
		c.Assert(err, IsNil)

		// Inspect test sdist
		f, err := files.OpenArtifactFile(testfile)
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		ins := pip.NewSdistInspector()
		a := metadata.NewArtifact()
		a.MimeType = mimetype.Lookup("application/gzip")
		a.SetRequestPending(ins, "test")

		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		if tc.approved {
			c.Assert(a.Approved(), Equals, true)
		} else {
			c.Assert(a.Rejected(), Equals, true)
			c.Assert(a.Metadata, DeepEquals, metadata.Metadata{})
		}
	}
}
