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
	"os"
	"path/filepath"

	"github.com/gabriel-vasile/mimetype"
	"github.com/go-mmap/mmap"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors"
	"github.com/canonical/fetch-service/inspectors/pip"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type metadataSuite struct{}

func (t *metadataSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&metadataSuite{})

func (s *metadataSuite) TestMetadataInspectorInterface(c *C) {
	var iface inspectors.Inspector
	ins := pip.NewMetadataInspector()
	c.Assert(ins, Implements, &iface)

}

func (s *metadataSuite) TestMetadataInspectorID(c *C) {
	ins := pip.NewMetadataInspector()
	c.Assert(ins.ID(), Equals, "pip.metadata")

}

func (s *metadataSuite) TestInspectRequest(c *C) {
	for _, tc := range []struct {
		url      string
		approved bool
	}{
		{"https://files.pythonhosted.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.whl.metadata", true},
		{"http://files.pythonhosted.org/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.whl.metadata", false},
		{"https://files.pythonhosted.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789a/foobar-1.0.0.whl.metadata", false},
		{"https://files.pythonhosted.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.whl", false},
		{"https://files.pythonhosted.org:443/packages/0f9a0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.whl.metadata", false},
		{"https://pypi.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.whl.metadata", false},
		{"ahttps://files.pythonhosted.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.whl.metadata", false},
	} {
		ins := pip.NewMetadataInspector()
		a := metadata.NewArtefact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, tc.approved)
		if tc.approved {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		}
	}
}

func (s *metadataSuite) TestInspectArtefactBadType(c *C) {
	ins := pip.NewMetadataInspector()
	a := metadata.NewArtefact()
	a.MimeType = mimetype.Lookup("application/zip")

	err := ins.InspectArtefact(nil, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
	c.Assert(a.Rejected(), Equals, true)
}

func (s *metadataSuite) TestMetadataInspectArtefactReadMetadata(c *C) {
	tmp := c.MkDir()
	testfile := filepath.Join(tmp, "test.metadata")

	content := "Metadata-Version: 2.1\n" +
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

	// Create test metadata
	err := os.WriteFile(testfile, []byte(content), 0644)
	c.Assert(err, IsNil)

	// Inspect test metadata
	f, err := mmap.Open(testfile)
	c.Assert(err, IsNil)

	ins := pip.NewMetadataInspector()
	a := metadata.NewArtefact()
	a.MimeType = mimetype.Lookup("text/plain")
	a.SetRequestOpinion(ins.ID(), opinions.Pending, "test")

	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, true)
	c.Assert(a.Metadata.Name, Equals, "metadata file for package 'craft-parts'")
	c.Assert(a.Metadata.Version, Equals, "1.31.0")
	c.Assert(a.Metadata.Description, Equals, "Python metadata file")
	c.Assert(a.Metadata.Vendor, Equals, "Canonical Ltd.")
	c.Assert(a.Metadata.Author, Equals, "Canonical Ltd.")
	c.Assert(a.Metadata.AuthorEmail, Equals, "snapcraft@lists.snapcraft.io")
	c.Assert(a.ResponseInspection["pip.metadata"], DeepEquals, &metadata.Inspection{
		Opinion: opinions.Approved,
		Reason:  "metadata file successfully parsed",
		Annotations: metadata.Annotation{
			"metadata-version": "2.1",
		},
	})
}

func (s *metadataSuite) TestMetadataInspectArtefactBadFormat(c *C) {
	tmp := c.MkDir()
	testfile := filepath.Join(tmp, "test.metadata")

	content := "something else"

	// Create test metadata
	err := os.WriteFile(testfile, []byte(content), 0644)
	c.Assert(err, IsNil)

	// Inspect test metadata
	f, err := mmap.Open(testfile)
	c.Assert(err, IsNil)
	defer f.Close()

	ins := pip.NewMetadataInspector()
	a := metadata.NewArtefact()
	a.MimeType = mimetype.Lookup("text/plain")
	a.SetRequestOpinion(ins.ID(), opinions.Pending, "test")

	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Rejected(), Equals, true)
}
