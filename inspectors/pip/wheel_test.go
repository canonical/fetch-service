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
	"testing"

	"github.com/go-mmap/mmap"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors"
	"github.com/canonical/fetch-service/inspectors/pip"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/testutils"
)

type wheelSuite struct{}

func (t *wheelSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&wheelSuite{})

func Test(t *testing.T) { TestingT(t) }

const (
	whlURL = "https://files.pythonhosted.org/packages/1a/27/39933dc51320918ca559eb1abb2ab6d4083f431f1e755c0e79cc717494d7/craft_grammar-1.1.1-py2.py3-none-any.whl"
)

func (s *wheelSuite) TestWheelInspectorInterface(c *C) {
	var iface inspectors.Inspector
	ins := pip.NewWheelInspector()
	c.Assert(ins, Implements, &iface)

}

func (s *wheelSuite) TestWheelInspectorID(c *C) {
	ins := pip.NewWheelInspector()
	c.Assert(ins.ID(), Equals, "pip.wheel")

}

func (s *wheelSuite) TestInspectRequest(c *C) {
	for _, tc := range []struct {
		url      string
		approved bool
	}{
		{"https://files.pythonhosted.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.whl", true},
		{"http://files.pythonhosted.org/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.whl", false},
		{"https://files.pythonhosted.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789a/foobar-1.0.0.whl", false},
		{"https://files.pythonhosted.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.tar.gz", false},
		{"https://files.pythonhosted.org:443/packages/0f9a0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.whl", false},
		{"https://pypi.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.whl", false},
		{"ahttps://files.pythonhosted.org:443/packages/0f/9a/0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab/foobar-1.0.0.whl", false},
	} {
		ins := pip.NewWheelInspector()
		a := metadata.NewArtefact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		_, ok := a.ApprovedReqs[ins.ID()]
		c.Assert(ok, Equals, tc.approved)
	}
}

func (s *wheelSuite) TestInspectArtefactBadType(c *C) {
	ins := pip.NewWheelInspector()
	a := metadata.NewArtefact()
	a.Metadata.Type = "application/zip"

	err := ins.InspectArtefact(nil, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
	c.Assert(a.Rejected(), Equals, false)
}

func (s *wheelSuite) TestWheelInspectArtefactBadContent(c *C) {
	tmp := c.MkDir()
	zipfile := filepath.Join(tmp, "test.whl")
	zdir := filepath.Join(tmp, "root")

	err := writeFile(zdir, "testwheel-1.0.dist-info/METADATA", "metadata")
	c.Assert(err, IsNil)

	err = writeFile(zdir, "testwheel-1.0.dist-info/RECORD", "record")
	c.Assert(err, IsNil)

	err = writeFile(zdir, "testwheel-1.0.dist-info/WHEEL", "wheel")
	c.Assert(err, IsNil)

	err = testutils.CreateZip(zipfile, zdir)
	c.Assert(err, IsNil)

	f, err := mmap.Open(zipfile)
	c.Assert(err, IsNil)
	defer f.Close()

	ins := pip.NewWheelInspector()
	a := metadata.NewArtefact()
	a.Metadata.Type = "application/x.python.wheel"

	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Rejected(), Equals, true)
	c.Assert(a.Opinions, DeepEquals, []metadata.Opinion{
		metadata.Opinion{
			InspectorID: "pip.wheel",
			Opinion:     metadata.Rejected,
			Reason:      "wheel name not found",
		},
		metadata.Opinion{
			InspectorID: "pip.wheel",
			Opinion:     metadata.Rejected,
			Reason:      "wheel version not found",
		},
		metadata.Opinion{
			InspectorID: "pip.wheel",
			Opinion:     metadata.Rejected,
			Reason:      "wheel metadata version not found",
		},
		metadata.Opinion{
			InspectorID: "pip.wheel",
			Opinion:     metadata.Rejected,
			Reason:      "malformed RECORD entry: 'record'",
		},
	})
}

func (s *wheelSuite) TestWheelReadMetadata(c *C) {
	tmp := c.MkDir()
	zipfile := filepath.Join(tmp, "test.whl")
	zdir := filepath.Join(tmp, "root")

	err := writeFile(zdir, "testwheel-1.0.dist-info/METADATA", "Metadata-Version: 2.1\n"+
		"Name: trololo\n"+
		"Version: 3.14159\n"+
		"Summary: A trove of marvelous thingies\n"+
		"Author: Poppy Bolger\n"+
		"Author-email: contact@foobar.com\n"+
		"Random-tag: doesn't matter\n"+
		"License: 3-Clause BSD\n"+
		"\n"+
		"Lorem ipsum dolor sit amet,\n"+
		"consectetur adipiscing elit.\n")
	c.Assert(err, IsNil)

	err = testutils.CreateZip(zipfile, zdir)
	c.Assert(err, IsNil)

	f, err := mmap.Open(zipfile)
	c.Assert(err, IsNil)
	defer f.Close()

	ins := pip.NewWheelInspector()
	a := metadata.NewArtefact()
	a.Metadata.Type = "application/x.python.wheel"

	ver, err := pip.ReadWheelMetadata(ins, f, int64(f.Len()), a)
	c.Assert(err, IsNil)
	c.Assert(ver, Equals, "2.1")
	c.Assert(a.Metadata.Name, Equals, "trololo")
	c.Assert(a.Metadata.Version, Equals, "3.14159")
	c.Assert(a.Metadata.Description, Equals, "A trove of marvelous thingies")
	c.Assert(a.Metadata.Vendor, Equals, "Poppy Bolger")
	c.Assert(a.Metadata.Author, Equals, "Poppy Bolger")
	c.Assert(a.Metadata.AuthorEmail, Equals, "contact@foobar.com")
	c.Assert(a.Metadata.License, Equals, "BSD-3-Clause")
}

func (s *wheelSuite) TestReadWheelRecord(c *C) {
	for _, tc := range []struct {
		record   string
		opinions []metadata.Opinion
	}{
		{
			// happy case
			record:   "foobar/somefile,sha256=cv3m7lT96eh7Jn_TPtdhxFzCusNN-nlFMmiAeq1TGOQ,54",
			opinions: []metadata.Opinion{},
		},
		{
			// size mismatch
			record: "foobar/somefile,sha256=cv3m7lT96eh7Jn_TPtdhxFzCusNN-nlFMmiAeq1TGOQ,55",
			opinions: []metadata.Opinion{
				metadata.Opinion{
					InspectorID: "pip.wheel",
					Opinion:     metadata.Rejected,
					Reason:      "foobar/somefile: file size 54 does not match recorded size 55",
				},
			},
		},
		{
			// digest mismatch
			record: "foobar/somefile,sha256=cv3m7lT96eh7Jn_TPtdhxFzCusNN-nlFMmiAeq1TGOP,54",
			opinions: []metadata.Opinion{
				metadata.Opinion{
					InspectorID: "pip.wheel",
					Opinion:     metadata.Rejected,
					Reason:      "foobar/somefile: digest mismatch",
				},
			},
		},
		{
			// malformed record entry
			record: "foobar/somefile\n",
			opinions: []metadata.Opinion{
				metadata.Opinion{
					InspectorID: "pip.wheel",
					Opinion:     metadata.Rejected,
					Reason:      "malformed RECORD entry: 'foobar/somefile'",
				},
			},
		},
		{
			// unknown digest type
			record: "foobar/somefile,sha1=cv3m7lT96eh7Jn_TPtdhxFzCusNN-nlFMmiAeq1TGOP,54",
			opinions: []metadata.Opinion{
				metadata.Opinion{
					InspectorID: "pip.wheel",
					Opinion:     metadata.Rejected,
					Reason:      "foobar/somefile: unknown digest type 'sha1'",
				},
			},
		},
		{
			// bad sha256 digest
			record: "foobar/somefile,sha256=abcd!!!,54",
			opinions: []metadata.Opinion{
				metadata.Opinion{
					InspectorID: "pip.wheel",
					Opinion:     metadata.Rejected,
					Reason:      "foobar/somefile: digest decode error: illegal base64 data at input byte 4",
				},
			},
		},
		{
			// unlisted file
			record: "",
			opinions: []metadata.Opinion{
				metadata.Opinion{
					InspectorID: "pip.wheel",
					Opinion:     metadata.Rejected,
					Reason:      "foobar/somefile: file not in RECORD",
				},
			},
		},
	} {

		tmp := c.MkDir()
		zipfile := filepath.Join(tmp, "test.whl")
		zdir := filepath.Join(tmp, "root")

		err := writeFile(zdir, "foobar/somefile", "Lorem ipsum dolor sit amet consectetur adipiscing elit")
		c.Assert(err, IsNil)

		record := tc.record
		if record != "" {
			record += "\n"
		}

		err = writeFile(zdir, "testwheel-1.0.dist-info/RECORD", record+
			"testwheel-1.0.dist-info/RECORD,,\n")
		c.Assert(err, IsNil)

		err = testutils.CreateZip(zipfile, zdir)
		c.Assert(err, IsNil)

		f, err := mmap.Open(zipfile)
		c.Assert(err, IsNil)
		defer f.Close()

		ins := pip.NewWheelInspector()
		a := metadata.NewArtefact()
		files, err := pip.ListWheelFiles(ins, f, int64(f.Len()), a)
		c.Assert(err, IsNil)

		h, _ := metadata.NewSha256Digest("72fde6ee54fde9e87b267fd33ed761c45cc2bac34dfa79453268807aad5318e4")
		c.Assert(len(files), Equals, 2)
		c.Assert(pip.MemberFile(files[0]), DeepEquals, pip.MemberFile{Name: "foobar/somefile", Sha256: h, Size: 54})

		err = pip.ReadWheelRecord(ins, f, int64(f.Len()), a, files)
		c.Assert(err, IsNil)
		c.Assert(a.Opinions, DeepEquals, tc.opinions)
	}
}

func (s *wheelSuite) TestWheelInspectArtefact(c *C) {
	tmp := c.MkDir()
	filename := filepath.Join(tmp, "wheel.data")
	err := testutils.HTTPDownload(whlURL, filename)
	c.Assert(err, IsNil)

	a := metadata.NewArtefact()
	a.Metadata.Type = "application/x.python.wheel"

	f, err := mmap.Open(filename)
	c.Assert(err, IsNil)
	defer f.Close()

	ins := pip.NewWheelInspector()
	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)

	c.Check(a.Metadata.Type, Equals, "application/x.python.wheel")
	c.Check(a.Metadata.Name, Equals, "craft-grammar")
	c.Check(a.Metadata.Version, Equals, "1.1.1")
	c.Check(a.Metadata.Vendor, Equals, "Canonical Ltd.")
	c.Check(a.Metadata.Description, Equals, `"Advance Grammar for Craft Parts"`)
	c.Check(a.Metadata.Author, Equals, "Canonical Ltd.")
	c.Check(a.Metadata.AuthorEmail, Equals, "snapcraft@lists.snapcraft.io")
	c.Check(a.Metadata.License, Equals, "LGPL-3")
	c.Assert(a.Approved(), Equals, true)
}

func writeFile(dir, name, data string) error {
	if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(data), 0644)
}
