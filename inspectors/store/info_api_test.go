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
	bconfig "github.com/canonical/fetch-service/inspectors/bldbin/config"
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
			glob.MustCompile("https://api.snapcraft.io:443/v2/revisions/resolve"),
			glob.MustCompile("https://api.snapcraft.io:443/v1/craft/**"),
			glob.MustCompile("https://dashboard.snapcraft.io:443/site_media/appmedia/**/*.png"),
		},
	}
}

func getTestBldbinInspectorConfig() bconfig.BldBinInspectorConfig {
	return bconfig.BldBinInspectorConfig{
		Urls: []glob.Glob{
			glob.MustCompile("https://api.snapcraft.io:443/api/v1/bins/download/**"),
		},
	}
}

func (s *storeSuite) TestStoreInfoApiInspectorID(c *C) {
	ins := store.NewStoreInfoAPIInspector(getTestStoreInspectorConfig(), getTestBldbinInspectorConfig())
	c.Assert(ins.ID(), Equals, "store.info-api")
}

type storeInfoApiInspectRequestTest struct {
	url     string // The info request URL
	pending bool   // Whether the inspection result should be pending
}

var storeInfoApiInspectRequestTests = []storeInfoApiInspectRequestTest{{
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
}, {
	url:     "https://api.snapcraft.io:443/api/v1/bins/download/w0VWGQnkqH0EDK7aOda6x9ZP5rHsAT4b_1.bin",
	pending: true,
}}

func (s *storeSuite) TestStoreInfoApiInspectRequest(c *C) {
	for _, tc := range storeInfoApiInspectRequestTests {
		ins := store.NewStoreInfoAPIInspector(getTestStoreInspectorConfig(), getTestBldbinInspectorConfig())
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

func (s *storeSuite) TestStoreInfoApiArtifactInspector(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/json"
	a.Metadata.Size = 1743

	f, err := files.OpenArtifactFile("testdata/info.json")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := store.NewStoreInfoAPIInspector(getTestStoreInspectorConfig(), getTestBldbinInspectorConfig())
	a.SetRequestPending(ins, "test").Annotate(Annotation{"type": "bins"})
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Approved(), Equals, true)
	c.Check(a.Metadata.Type, Equals, "application/x.canonical.store.info-api")
	c.Check(a.Metadata.Name, Equals, "Store protocol response")
	c.Check(a.Metadata.Size, Equals, int64(1743))
	c.Check(a.Metadata.Description, Equals, "Store response for info request")
	c.Check(a.ResponseInspection["store.info-api"].Annotations, DeepEquals, Annotation{
		"name":       "starcraft-patchelf",
		"type":       "bins",
		"publisher":  "Imani Pelton",
		"package-id": "w0VWGQnkqH0EDK7aOda6x9ZP5rHsAT4b",
	})

	// Check inspector state
	const sha = "b162cad83a53c5190a249ed0fd3b80c5ff89454654541efe918bbe23883d47541b1bb0a572b994caa828c8982dcf0bdd"
	ainfo, revision, channel := store.StoreInfoAPIInspectorFindInfo(ins, sha)
	c.Check(ainfo.Type, Equals, "bins")
	c.Check(ainfo.Publisher, Equals, "Imani Pelton")
	c.Check(ainfo.ID, Equals, "w0VWGQnkqH0EDK7aOda6x9ZP5rHsAT4b")
	c.Check(ainfo.RevInfo, HasLen, 1)
	c.Check(ainfo.RevInfo[sha].Sha3_384, Equals, sha)
	c.Check(ainfo.RevInfo[sha].Size, Equals, uint64(858288))
	c.Check(ainfo.RevInfo[sha].Revision, Equals, "1")
	c.Check(ainfo.RevInfo[sha].Channel, Equals, "latest/edge")
	c.Check(revision, Equals, "1")
	c.Check(channel, Equals, "latest/edge")
}

func (s *storeSuite) TestStoreInfoApiArtifactBadType(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "text/plain"
	a.Metadata.Size = 1743

	f, err := files.OpenArtifactFile("testdata/info.json")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := store.NewStoreInfoAPIInspector(getTestStoreInspectorConfig(), getTestBldbinInspectorConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Approved(), Equals, false)
}

func (s *storeSuite) TestStoreInfoApiArtifactBadContent(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/json"
	a.Metadata.Size = 1743

	f := strings.NewReader(`{"content": "bad"}`)

	ins := store.NewStoreInfoAPIInspector(getTestStoreInspectorConfig(), getTestBldbinInspectorConfig())
	err := ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Approved(), Equals, false)
}

type validateBinTest struct {
	infoType string               // The file type according to the info API
	opinion  opinions.OpinionKind // The expected inspection opinion
	reason   string               // The expected inspection reason
}

var validateBinTests = []validateBinTest{{
	infoType: "bins",
	opinion:  opinions.Unknown,
	reason:   "file digest matches store info API bin request",
}, {
	infoType: "not-bins",
	opinion:  opinions.Rejected,
	reason:   "file digest matches a request for a different package type",
}}

func (s *storeSuite) TestValidateBin(c *C) {
	for _, tc := range validateBinTests {
		a := metadata.NewArtifact()
		a.Metadata.Type = "application/x-xz"
		a.Metadata.Size = 2156

		f, err := files.OpenArtifactFile("testdata/starcraft-test-2.0.0.tar.xz")
		c.Assert(err, IsNil)

		ainfo := &store.StoreInfoAPIInfo{
			Type:      tc.infoType,
			ID:        "pkgid",
			Publisher: "publisher",
			RevInfo: map[string]store.StoreInfoAPIRevisionInfo{
				testDigest: {
					Sha3_384: testDigest,
					Size:     2156,
					Revision: "1",
					Channel:  "latest/stable",
				},
			},
		}

		ins := store.NewStoreInfoAPIInspector(getTestStoreInspectorConfig(), getTestBldbinInspectorConfig())
		store.StoreInfoAPIInspectorSetInfo(ins, "pkgid", ainfo)

		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Check(a.Rejected(), Equals, true) // The store bin inspetor doesn't approve
		c.Check(a.ResponseInspection["store.info-api"].Opinion, Equals, tc.opinion)
		c.Check(a.ResponseInspection["store.info-api"].Reason, Equals, tc.reason)
	}

}

type validateBinInvalidFormatTest struct {
	filename string // The artifact file to be tested
}

var validateBinInvalidFormatTests = []validateBinInvalidFormatTest{{
	filename: "testdata/info.json", // Invalid format
}, {
	filename: "testdata/invalid-bin.tar.xz", // Invalid metadata
}, {
	filename: "testdata/not-a-tarball.xz", // non-tarball xz archive
}}

func (s *storeSuite) TestValidateBinInvalid(c *C) {
	for _, tc := range validateBinInvalidFormatTests {
		a := metadata.NewArtifact()
		a.Metadata.Type = "application/x-xz"
		a.Metadata.Size = 2156

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer f.Close()

		ins := store.NewStoreInfoAPIInspector(getTestStoreInspectorConfig(), getTestBldbinInspectorConfig())

		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Check(a.ResponseInspection, HasLen, 0)
	}
}

func (s *storeSuite) TestValidateBinNoRequest(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/x-xz"
	a.Metadata.Size = 2156

	f, err := files.OpenArtifactFile("testdata/starcraft-test-2.0.0.tar.xz")
	c.Assert(err, IsNil)

	ins := store.NewStoreInfoAPIInspector(getTestStoreInspectorConfig(), getTestBldbinInspectorConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Rejected(), Equals, true)
	c.Check(a.ResponseInspection["store.info-api"].Reason, Equals, "file digest does not match any store info API request")

}

func (s *storeSuite) TestSha3_384Digest(c *C) {
	f, err := os.Open("testdata/starcraft-test-2.0.0.tar.xz")
	c.Assert(err, IsNil)
	defer f.Close()

	d, err := store.Sha3_384Digest(f)
	c.Assert(err, IsNil)
	c.Check(d, Equals, testDigest)
}
