// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2025 Canonical Ltd.
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

package bldbin_test

import (
	"testing"

	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/inspectors/bldbin"
	"github.com/canonical/fetch-service/inspectors/bldbin/config"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/store"
	storeConfig "github.com/canonical/fetch-service/inspectors/store/config"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type bldbinSuite struct {
}

var _ = Suite(&bldbinSuite{})

func Test(t *testing.T) { TestingT(t) }

func getTestBldBinConfig() config.BldBinInspectorConfig {
	return config.BldBinInspectorConfig{
		URLs: []glob.Glob{glob.MustCompile("https://api.snapcraft.io:443/api/v1/bins/download/**")},
	}
}

func getTestStoreConfig() storeConfig.StoreInspectorConfig {
	return storeConfig.StoreInspectorConfig{
		URLs: []glob.Glob{
			glob.MustCompile("https://api.snapcraft.io:443/v2/bins/info/**"),
		},
	}
}

func (s *bldbinSuite) TestBldBinInspectorID(c *C) {
	ins := bldbin.NewBldBinInspector(getTestBldBinConfig())
	c.Assert(ins.ID(), Equals, "bld.bin")
}

func (s *bldbinSuite) TestBldBinInspectorInterface(c *C) {
	var iface Inspector
	ins := bldbin.NewBldBinInspector(config.BldBinInspectorConfig{})
	c.Assert(ins, Implements, &iface)

}

type bldbinInspectRequestTest struct {
	url     string // The request URL
	pending bool   // Whether this should be set to pending state
}

var bldbinInspectRequestTests = []bldbinInspectRequestTest{{
	url:     "https://api.snapcraft.io:443/api/v1/bins/download/w0VWGQnkqH0EDK7aOda6x9ZP5rHsAT4b_1.bin",
	pending: true,
}, {
	url:     "https://api.snapcraft.io:443/api/bins/download/w0VWGQnkqH0EDK7aOda6x9ZP5rHsAT4b_1.bin",
	pending: false,
}, {
	url:     "https://not-api.snapcraft.io:443/api/bins/download/w0VWGQnkqH0EDK7aOda6x9ZP5rHsAT4b_1.bin",
	pending: false,
}, {
	url:     "http://api.snapcraft.io/api/v1/bins/download/w0VWGQnkqH0EDK7aOda6x9ZP5rHsAT4b_1.bin",
	pending: false,
}}

func (s *bldbinSuite) TestBldBinInspectRequest(c *C) {
	for _, tc := range bldbinInspectRequestTests {
		ins := bldbin.NewBldBinInspector(getTestBldBinConfig())
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

type bldbinArtifactInspectorTest struct {
	filename string // The bldbin file to be inspected
	approved bool   // Whether it should be approved
}

var bldbinArtifactInspectorTests = []bldbinArtifactInspectorTest{{
	filename: "testdata/starcraft-test-2.0.0.tar.xz",
	approved: true,
}, {
	filename: "testdata/invalid-bin.tar.xz",
	approved: false,
}, {
	filename: "testdata/Packages.xz",
	approved: false,
}}

func (s *bldbinSuite) TestBldBinArtifactInspector(c *C) {
	for _, tc := range bldbinArtifactInspectorTests {
		a := metadata.NewArtifact()
		a.Metadata.Type = "application/x-xz"
		a.MimeType = mimetype.Lookup("application/x-xz")

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		storeIns := store.NewStoreInfoAPIInspector(getTestStoreConfig(), getTestBldBinConfig())
		ins := bldbin.NewBldBinInspector(getTestBldBinConfig())
		a.SetRequestPending(storeIns, "test").Annotate(Annotation{"package-id": "package-id"})
		a.SetResponseUnknown(storeIns, "test").Annotate(Annotation{"revision": "1234"})
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Assert(a.Approved(), Equals, tc.approved)

		if tc.approved {
			c.Check(a.ResponseApproved(), Equals, true)
			c.Check(a.Metadata.Name, Equals, "starcraft-test")
			c.Check(a.Metadata.Version, Equals, "2.0.0")
			c.Check(a.Metadata.Description, Equals, "Package used in Starcraft tests")
			c.Check(a.Metadata.Vendor, Equals, "")
			c.Check(a.Metadata.Author, Equals, "")
			c.Check(a.Metadata.License, Equals, "GPL-3.0-or-later")
			c.Check(a.Metadata.Architecture, Equals, "amd64")
			c.Check(a.Metadata.StoreRevision, Equals, "1234")
			c.Check(a.Metadata.ContentID, Equals, "package-id")
		} else {
			// We don't recognize this artifact
			c.Check(a.ResponseApproved(), Equals, false)
			c.Check(a.ResponseRejected(), Equals, false)
		}
	}
}
