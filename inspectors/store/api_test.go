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

package store_test

import (
	"os"
	"strings"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/glob"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/store"
	"github.com/canonical/fetch-service/inspectors/store/config"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

const (
	testDigest = "6c9a52494c347bc1a32be8c9b6a1441f8325305db0f0b99a2c39f7ec23747315c8a66b9b51c84abf80d30025db35d6f6"
)

type storeSuite struct {
}

var _ = Suite(&storeSuite{})

func Test(t *testing.T) { TestingT(t) }

func getTestStoreInspectorConfig() config.StoreInspectorConfig {
	return config.StoreInspectorConfig{
		Urls: []glob.Glob{
			glob.MustCompile("https://api.snapcraft.io:443/v2/bins/info/**"),
		},
	}
}

func (s *storeSuite) TestStoreApiInspectorID(c *C) {
	ins := store.NewStoreApiInspector(getTestStoreInspectorConfig())
	c.Assert(ins.ID(), Equals, "store.api")
}

type storeApiInspectRequestTest struct {
	url     string // The info request URL
	pending bool   // Whether the inspection result should be pending
}

var storeApiInspectRequestTests = []storeApiInspectRequestTest{{
	url:     "https://api.snapcraft.io:443/v2/bins/info/package-name",
	pending: true,
}, {
	url:     "https://api.snapcraft.io:443/v2/bins/info", // No package name
	pending: false,
}, {
	url:     "https://api.snapcraft.io:443/v1/bins/info", // Wrong version
	pending: false,
}, {
	url:     "https://api.snapcraft.io:443/v2/bins/refresh", // Wrong path
	pending: false,
}, {
	url:     "https://api.snapcraft.io:443/v2/snaps/info", // Wrong package type
	pending: false,
}, {
	url:     "http://api.snapcraft.io/v2/snaps/info", // Wrong protocol
	pending: false,
}}

func (s *storeSuite) TestStoreApiInspectRequest(c *C) {
	for _, tc := range storeApiInspectRequestTests {
		ins := store.NewStoreApiInspector(getTestStoreInspectorConfig())
		a := metadata.NewArtifact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, tc.pending)
		if tc.pending {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		}
	}
}

func (s *storeSuite) TestStoreApiArtifactInspector(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/json"
	a.Metadata.Size = 1743

	f, err := files.OpenArtifactFile("testdata/info.json")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := store.NewStoreApiInspector(getTestStoreInspectorConfig())
	a.SetRequestPending(ins, "test").Annotate(Annotation{"type": "bins"})
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Approved(), Equals, true)
	c.Check(a.Metadata.Type, Equals, "application/x.canonical.store-api")
	c.Check(a.Metadata.Name, Equals, "Store protocol response")
	c.Check(a.Metadata.Size, Equals, int64(1743))
	c.Check(a.Metadata.Description, Equals, "Store response for info request")
	c.Check(a.ResponseInspection["store.api"].Annotations, DeepEquals, Annotation{
		"name":       "starcraft-patchelf",
		"type":       "bins",
		"publisher":  "Imani Pelton",
		"package-id": "w0VWGQnkqH0EDK7aOda6x9ZP5rHsAT4b",
	})

	// Check inspector state
	const sha = "b162cad83a53c5190a249ed0fd3b80c5ff89454654541efe918bbe23883d47541b1bb0a572b994caa828c8982dcf0bdd"
	ainfo, revision, channel := store.StoreApiInspectorFindStoreApiInfo(ins, sha)
	c.Check(ainfo.Type, Equals, "bins")
	c.Check(ainfo.Publisher, Equals, "Imani Pelton")
	c.Check(ainfo.ID, Equals, "w0VWGQnkqH0EDK7aOda6x9ZP5rHsAT4b")
	c.Check(ainfo.RevInfo, DeepEquals, []store.StoreApiRevisionInfo{
		{
			Sha3_384: sha,
			Size:     858288,
			Revision: "1",
			Channel:  "latest/edge",
		},
	})
	c.Check(revision, Equals, "1")
	c.Check(channel, Equals, "latest/edge")
}

func (s *storeSuite) TestStoreApiArtifactBadType(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "text/plain"
	a.Metadata.Size = 1743

	f, err := files.OpenArtifactFile("testdata/info.json")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := store.NewStoreApiInspector(getTestStoreInspectorConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Approved(), Equals, false)
}

func (s *storeSuite) TestStoreApiArtifactBadContent(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "text/plain"
	a.Metadata.Size = 1743

	f := strings.NewReader(`{"content": "bad"}`)

	ins := store.NewStoreApiInspector(getTestStoreInspectorConfig())
	err := ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Approved(), Equals, false)
}

func (s *storeSuite) TestValidateBin(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/x-xz"
	a.Metadata.Size = 2156

	f, err := files.OpenArtifactFile("testdata/starcraft-test-2.0.0.tar.xz")

	ainfo := &store.StoreApiInfo{
		Type:      "bins",
		ID:        "pkgid",
		Publisher: "publisher",
		RevInfo: []store.StoreApiRevisionInfo{
			{
				Sha3_384: testDigest,
				Size:     2156,
			},
		},
	}

	ins := store.NewStoreApiInspector(getTestStoreInspectorConfig())
	store.StoreApiInspectorSetStoreApiInfo(ins, "pkgid", ainfo)

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Rejected(), Equals, true) // The store bin inspetor doesn't approve
	c.Check(a.ResponseInspection["store.api"].Opinion, Equals, opinions.Unknown)
	c.Check(a.ResponseInspection["store.api"].Reason, Equals, "file digest matches store API bin request")

}

type validateBinInvalidFormatTest struct {
	filename string
}

var validateBinInvalidFormatTests = []validateBinInvalidFormatTest{{
	filename: "testdata/info.json", // Invalid format
}, {
	filename: "testdata/invalid-bin.tar.xz", // Invalid metadata
}}

func (s *storeSuite) TestValidateBinInvalid(c *C) {
	for _, tc := range validateBinInvalidFormatTests {
		a := metadata.NewArtifact()
		a.Metadata.Type = "application/x-xz"
		a.Metadata.Size = 2156

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer f.Close()

		ins := store.NewStoreApiInspector(getTestStoreInspectorConfig())

		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Check(a.Rejected(), Equals, true)
	}
}

func (s *storeSuite) TestValidateBinNoRequest(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/x-xz"
	a.Metadata.Size = 2156

	f, err := files.OpenArtifactFile("testdata/starcraft-test-2.0.0.tar.xz")

	ins := store.NewStoreApiInspector(getTestStoreInspectorConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Rejected(), Equals, true)
	c.Check(a.ResponseInspection["store.api"].Reason, Equals, "file digest does not match any store API request")

}

func (s *storeSuite) TestSha3_384Digest(c *C) {
	f, err := os.Open("testdata/starcraft-test-2.0.0.tar.xz")
	c.Assert(err, IsNil)
	defer f.Close()

	d, err := store.Sha3_384Digest(f)
	c.Assert(err, IsNil)
	c.Check(d, Equals, testDigest)
}
