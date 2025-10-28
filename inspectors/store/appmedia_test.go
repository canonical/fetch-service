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
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/store"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

func (s *storeSuite) TestStoreAppMediaInspectorID(c *C) {
	ins := store.NewStoreAppMediaInspector(getTestStoreInspectorConfig())
	c.Assert(ins.ID(), Equals, "store.appmedia")
}

type storeAppMediaInspectRequestTest struct {
	url     string // The info request URL
	pending bool   // Whether the inspection result should be pending
}

var storeAppMediaInspectRequestTests = []storeAppMediaInspectRequestTest{{
	url:     "https://dashboard.snapcraft.io:443/site_media/appmedia/2019/09/snapd.png",
	pending: true,
}, {
	url:     "https://dashboard.snapcraft.io:443/site_media/other/2019/09/snapd.png",
	pending: false,
}, {
	url:     "https://api.snapcraft.io:443/site_media/appmedia/2019/09/snapd.png",
	pending: false,
}, {
	url:     "http://dashboard.snapcraft.io/site_media/appmedia/2019/09/snapd.png",
	pending: false,
}}

func (s *storeSuite) TestStoreAppMediaInspectRequest(c *C) {
	for _, tc := range storeAppMediaInspectRequestTests {
		ins := store.NewStoreAppMediaInspector(getTestStoreInspectorConfig())
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

type storeAppMediaArtifactInspectorTest struct {
	filename string // The test file name
	pending  bool   // Whether the request was set to pending
	approved bool   // Whether the artifact should be approved
	reason   string // The reason for approval or rejection
}

var storeAppMediaArtifactInspectorTests = []storeAppMediaArtifactInspectorTest{{
	filename: "testdata/snapd.png",
	pending:  true,
	approved: true,
	reason:   "store media file in PNG format",
}, {
	filename: "testdata/snapd.png",
	pending:  false,
	approved: false,
	reason:   "unknown PNG image",
}, {
	filename: "testdata/resolve.json",
	pending:  true,
	approved: false,
	reason:   "", // unrecognized artifact
}}

func (s *storeSuite) TestStoreAppMediaArtifactInspector(c *C) {
	for _, tc := range storeAppMediaArtifactInspectorTests {
		a := metadata.NewArtifact()
		a.Metadata.Type = "image/png"
		a.Metadata.Size = 1234

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer f.Close()

		ins := store.NewStoreAppMediaInspector(getTestStoreInspectorConfig())
		if tc.pending {
			a.SetRequestPending(ins, "test")
		}
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Check(a.Approved(), Equals, tc.approved, Commentf("%+v", tc))

		if tc.approved {
			insp := a.ResponseInspection["store.appmedia"]
			c.Check(insp.Opinion, Equals, opinions.Approved)

			c.Check(insp.Reason, Equals, tc.reason)
			c.Check(insp.Annotations["width"], Equals, 460)
			c.Check(insp.Annotations["height"], Equals, 460)
			c.Check(a.Metadata.Type, Equals, "image/png")
			c.Check(a.Metadata.Name, Equals, "Image file")
			c.Check(a.Metadata.Size, Equals, int64(1234))
			c.Check(a.Metadata.Description, Equals, "Store media file in PNG format")
		}
	}
}

func (s *storeSuite) TestStoreAppMediaArtifactBadType(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "text/plain"
	a.Metadata.Size = 1743

	f, err := files.OpenArtifactFile("testdata/snapd.png")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := store.NewStoreInfoAPIInspector(getTestStoreInspectorConfig(), getTestBldbinInspectorConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Approved(), Equals, false)
}
