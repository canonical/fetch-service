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

package pip_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/pip"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/canonical/fetch-service/testutils"
)

type wheelSuite struct {
	slog logger.Logger
}

func (t *wheelSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&wheelSuite{logger.NewSessionLogger("test")})

func Test(t *testing.T) { TestingT(t) }

const (
	whlURL = "https://files.pythonhosted.org/packages/1a/27/39933dc51320918ca559eb1abb2ab6d4083f431f1e755c0e79cc717494d7/craft_grammar-1.1.1-py2.py3-none-any.whl"
)

func (s *wheelSuite) TestWheelInspectorInterface(c *C) {
	var iface Inspector
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
		{"https://files.pythonhosted.org:443/packages/81/d4/fd74714ed30a1dedd0b82427c02fa4deec64f173831ec716da11c51a50aa/MarkupSafe-2.1.5-cp38-cp38-manylinux_2_17_aarch64.manylinux2014_aarch64.whl", true},
		{"https://files.pythonhosted.org:443/packages/2d/0a/679461c511447ffaf176567d5c496d1de27cbe34a87df6677d7171b2fbd4/importlib_metadata-7.1.0-py3-none-any.whl", true},
		{"https://files.pythonhosted.org:443/packages/7f/66/b15ce62552d84bbfcec9a4873ab79d993a1dd4edb922cbfccae192bd5b5f/jaraco.classes-3.4.0-py3-none-any.whl", true},
	} {
		ins := pip.NewWheelInspector()
		a := metadata.NewArtifact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil, Commentf("test case: %+v", tc))

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, tc.approved, Commentf("test case: %+v", tc))
		if tc.approved {
			c.Assert(insp.Opinion, Equals, opinions.Unknown, Commentf("test case: %+v", tc))
			c.Assert(insp.Reason, Equals, "unsupported origin")
		}
	}
}

func (s *wheelSuite) TestInspectArtifactBadType(c *C) {
	ins := pip.NewWheelInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/octet-stream")

	err := ins.InspectArtifact(nil, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
	c.Assert(a.Rejected(), Equals, true)
}

func (s *wheelSuite) TestWheelInspectArtifactBadContent(c *C) {
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

	f, err := files.OpenArtifactFile(zipfile)
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	ins := pip.NewWheelInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/zip")
	a.SetRequestPending(ins, "test")

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Rejected(), Equals, true)
	c.Assert(a.ResponseInspection, DeepEquals, metadata.InspectionMap{
		"pip.wheel": &Inspection{
			Opinion: opinions.Rejected,
			Reason:  "wheel file requirements not met",
			Annotations: map[string]any{
				"files": 3,
				"faults": []string{
					"wheel name not found",
					"wheel version not found",
					"wheel metadata version not found",
				},
			},
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
		"Classifier: License :: OSI Approved :: GNU General Public License v3 or later (GPLv3+)\n"+
		"\n"+
		"Lorem ipsum dolor sit amet,\n"+
		"consectetur adipiscing elit.\n")
	c.Assert(err, IsNil)

	err = testutils.CreateZip(zipfile, zdir)
	c.Assert(err, IsNil)

	f, err := files.OpenArtifactFile(zipfile)
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	ins := pip.NewWheelInspector()
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/x.python.wheel"
	a.SetRequestPending(ins, "test")

	notes := pip.NewWheelNotes()
	err = pip.ReadWheelMetadata(ins, f, int64(f.Len()), a, notes, s.slog)
	c.Assert(err, IsNil)
	c.Assert(a.Metadata.Name, Equals, "trololo")
	c.Assert(a.Metadata.Version, Equals, "3.14159")
	c.Assert(a.Metadata.Description, Equals, "A trove of marvelous thingies")
	c.Assert(a.Metadata.Vendor, Equals, "Poppy Bolger")
	c.Assert(a.Metadata.Author, Equals, "Poppy Bolger")
	c.Assert(a.Metadata.AuthorEmail, Equals, "contact@foobar.com")
	c.Assert(a.Metadata.License, Equals, "GPL-3.0-or-later")
}

func (s *wheelSuite) TestReadWheelRecord(c *C) {
	for _, tc := range []struct {
		record     string
		inspection metadata.InspectionMap
	}{
		{
			// happy case
			record: "foobar/somefile,sha256=cv3m7lT96eh7Jn_TPtdhxFzCusNN-nlFMmiAeq1TGOQ,54",
			inspection: metadata.InspectionMap{
				"pip.wheel": &Inspection{
					Opinion: opinions.Approved,
					Reason:  "wheel file successfully parsed",
					Annotations: map[string]any{
						"files":                 2,
						"parsed-record-entries": 1,
					},
				},
			},
		},
		{
			// size mismatch
			record: "foobar/somefile,sha256=cv3m7lT96eh7Jn_TPtdhxFzCusNN-nlFMmiAeq1TGOQ,55",
			inspection: metadata.InspectionMap{
				"pip.wheel": &Inspection{
					Opinion: opinions.Rejected,
					Reason:  "wheel file parsed but failed integrity verification",
					Annotations: map[string]any{
						"files": 2,
						"faults": []string{
							"foobar/somefile: file size 54 does not match recorded size 55",
						},
					},
				},
			},
		},
		{
			// digest mismatch
			record: "foobar/somefile,sha256=cv3m7lT96eh7Jn_TPtdhxFzCusNN-nlFMmiAeq1TGOP,54",
			inspection: metadata.InspectionMap{
				"pip.wheel": &Inspection{
					Opinion: opinions.Rejected,
					Reason:  "wheel file parsed but failed integrity verification",
					Annotations: map[string]any{
						"files":  2,
						"faults": []string{"foobar/somefile: digest mismatch"},
					},
				},
			},
		},
		{
			// malformed record entry
			record: "foobar/somefile\n",
			inspection: metadata.InspectionMap{
				"pip.wheel": &Inspection{
					Opinion: opinions.Rejected,
					Reason:  "wheel file parsed but failed integrity verification",
					Annotations: map[string]any{
						"files":  2,
						"faults": []string{"malformed RECORD entry: 'foobar/somefile'"},
					},
				},
			},
		},
		{
			// unknown digest type
			record: "foobar/somefile,sha1=cv3m7lT96eh7Jn_TPtdhxFzCusNN-nlFMmiAeq1TGOP,54",
			inspection: metadata.InspectionMap{
				"pip.wheel": &Inspection{
					Opinion: opinions.Rejected,
					Reason:  "wheel file parsed but failed integrity verification",
					Annotations: map[string]any{
						"files":  2,
						"faults": []string{"foobar/somefile: unknown digest type 'sha1'"},
					},
				},
			},
		},
		{
			// bad sha256 digest
			record: "foobar/somefile,sha256=abcd!!!,54",
			inspection: metadata.InspectionMap{
				"pip.wheel": &Inspection{
					Opinion: opinions.Rejected,
					Reason:  "wheel file parsed but failed integrity verification",
					Annotations: map[string]any{
						"files": 2,
						"faults": []string{
							"foobar/somefile: digest decode error: illegal base64 data at input byte 4",
						},
					},
				},
			},
		},
		{
			// unlisted file
			record: "",
			inspection: metadata.InspectionMap{
				"pip.wheel": &Inspection{
					Opinion: opinions.Rejected,
					Reason:  "wheel file parsed but failed integrity verification",
					Annotations: map[string]any{
						"files":                 2,
						"parsed-record-entries": 0,
						"extra-files":           []string{"foobar/somefile"},
					},
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

		f, err := files.OpenArtifactFile(zipfile)
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		ins := pip.NewWheelInspector()
		a := metadata.NewArtifact()
		a.SetRequestPending(ins, "test")

		notes := pip.NewWheelNotes()
		files, err := pip.ListWheelFiles(ins, f, int64(f.Len()), a, notes)
		c.Assert(err, IsNil)

		h, _ := digests.NewSha256Digest("72fde6ee54fde9e87b267fd33ed761c45cc2bac34dfa79453268807aad5318e4")
		c.Assert(len(files), Equals, 2)
		c.Assert(pip.MemberFile(files[0]), DeepEquals, pip.MemberFile{Name: "foobar/somefile", Sha256: h, Size: 54})

		err = pip.ReadWheelRecord(ins, f, int64(f.Len()), a, files, notes)
		c.Assert(err, IsNil)

		pip.ProcessOpinion(ins, a, notes)

		c.Assert(a.ResponseInspection, DeepEquals, tc.inspection)
	}
}

func (s *wheelSuite) TestWheelInspectArtifact(c *C) {
	tmp := c.MkDir()
	filename := filepath.Join(tmp, "wheel.data")
	err := testutils.HTTPDownload(whlURL, filename)
	c.Assert(err, IsNil)

	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/zip")

	f, err := files.OpenArtifactFile(filename)
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	ins := pip.NewWheelInspector()
	a.SetRequestPending(ins, "test")
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	c.Assert(a.Approved(), Equals, true)
	c.Check(a.Metadata.Type, Equals, "application/x.python.wheel")
	c.Check(a.Metadata.Name, Equals, "craft-grammar")
	c.Check(a.Metadata.Version, Equals, "1.1.1")
	c.Check(a.Metadata.Vendor, Equals, "Canonical Ltd.")
	c.Check(a.Metadata.Description, Equals, `"Advance Grammar for Craft Parts"`)
	c.Check(a.Metadata.Author, Equals, "Canonical Ltd.")
	c.Check(a.Metadata.AuthorEmail, Equals, "snapcraft@lists.snapcraft.io")
	c.Check(a.Metadata.License, Equals, "LGPL-3")
}

func writeFile(dir, name, data string) error {
	if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(data), 0644)
}
