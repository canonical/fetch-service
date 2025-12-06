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

func (s *storeSuite) TestStoreTransformAPIInspectorID(c *C) {
	ins := store.NewStoreTransformsAPIInspector(getTestStoreInspectorConfig())
	c.Assert(ins.ID(), Equals, "store.transforms-api")
}

type storeTransformsAPIInspectRequestTest struct {
	url     string // The info request URL
	pending bool   // Whether the inspection result should be pending
}

var storeTransformsAPIInspectRequestTests = []storeTransformsAPIInspectRequestTest{{
	url:     "https://api.snapcraft.io:443/v1/craft/workspaces/1234/transforms",
	pending: true,
}, {
	url:     "https://api.snapcraft.io:443/v2/craft/workspaces/1234/transforms", // Wrong version
	pending: false,
}, {
	url:     "https://api.snapcraft.io:443/v1/craft/workspaces/1234/not-transforms", // Wrong path
	pending: false,
}, {
	url:     "http://api.snapcraft.io/v1/craft/workspaces/1234/not-transforms", // Wrong protocol
	pending: false,
}}

func (s *storeSuite) TestStoreTransformsAPIInspectRequest(c *C) {
	for _, tc := range storeTransformsAPIInspectRequestTests {
		ins := store.NewStoreTransformsAPIInspector(getTestStoreInspectorConfig())
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

type storeTransformsAPIArtifactInspectorTest struct {
	filename string // The test file name
	approved bool   // Whether the artifact should be approved
	reason   string // The reason for approval or rejection
}

var storeTransformsAPIArtifactInspectorTests = []storeTransformsAPIArtifactInspectorTest{{
	filename: "testdata/transforms.json",
	approved: true,
	reason:   "valid store transforms API response",
}, {
	filename: "testdata/resolve.json",
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/transforms-missing-field.json",
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/transforms-invalid-package-type.json",
	approved: false,
	reason:   "invalid package type",
}, {
	filename: "testdata/starcraft-test-2.0.0.tar.xz",
	approved: false,
	reason:   "", // unrecognized artifact
}}

func (s *storeSuite) TestStoreTransformsAPIArtifactInspector(c *C) {
	for _, tc := range storeTransformsAPIArtifactInspectorTests {
		a := metadata.NewArtifact()
		a.Metadata.Type = "application/json"
		a.Metadata.Size = 1234

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		ins := store.NewStoreTransformsAPIInspector(getTestStoreInspectorConfig())
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Check(a.Approved(), Equals, tc.approved, Commentf("%+v", tc))

		if tc.approved {
			insp := a.ResponseInspection["store.transforms-api"]
			c.Check(insp.Opinion, Equals, opinions.Approved)

			c.Check(insp.Reason, Equals, tc.reason)
			c.Check(insp.Annotations["workspace-id"], Equals, "1234")
			c.Check(insp.Annotations["transforms"], DeepEquals, []string{"starcraft-test from latest/beta to latest/edge"})
			c.Check(a.Metadata.Type, Equals, "application/x.canonical.store.transforms-api")
			c.Check(a.Metadata.Name, Equals, "Store protocol response")
			c.Check(a.Metadata.Size, Equals, int64(1234))
			c.Check(a.Metadata.Description, Equals, "Store response for workspace transforms request")
		}
	}
}

func (s *storeSuite) TestStoreTransformsAPIArtifactBadType(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "text/plain"
	a.Metadata.Size = 1743

	f, err := files.OpenArtifactFile("testdata/resolve.json")
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	ins := store.NewStoreInfoAPIInspector(getTestStoreInspectorConfig(), getTestBldbinInspectorConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Approved(), Equals, false)
}
