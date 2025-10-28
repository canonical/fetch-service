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

func (s *storeSuite) TestStoreResolveAPIInspectorID(c *C) {
	ins := store.NewStoreResolveAPIInspector(getTestStoreInspectorConfig())
	c.Assert(ins.ID(), Equals, "store.resolve-api")
}

type storeResolveAPIInspectRequestTest struct {
	url     string // The info request URL
	pending bool   // Whether the inspection result should be pending
}

var storeResolveAPIInspectRequestTests = []storeResolveAPIInspectRequestTest{{
	url:     "https://api.snapcraft.io:443/v2/revisions/resolve",
	pending: true,
}, {
	url:     "https://api.snapcraft.io:443/v2/revisions/resolve/package-name", // Extra elements at the end
	pending: false,
}, {
	url:     "https://api.snapcraft.io:443/v3/revisions/resolve", // Wrong version
	pending: false,
}, {
	url:     "https://api.snapcraft.io:443/v2/bins/info/package-name", // Different path
	pending: false,
}}

func (s *storeSuite) TestStoreResolveAPIInspectRequest(c *C) {
	for _, tc := range storeResolveAPIInspectRequestTests {
		ins := store.NewStoreResolveAPIInspector(getTestStoreInspectorConfig())
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

type storeResolveAPIArtifactInspectorTest struct {
	filename string // The test file name
	approved bool   // Whether the artifact should be approved
	reason   string // The reason for approval or rejection
}

var storeResolveAPIArtifactInspectorTests = []storeResolveAPIArtifactInspectorTest{{
	filename: "testdata/resolve.json",
	approved: true,
	reason:   "valid store resolve_revisions API response",
}, {
	filename: "testdata/resolve-empty.json",
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/resolve-missing-field.json",
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/resolve-invalid-namespace.json",
	approved: false,
	reason:   "invalid namespace",
}, {
	filename: "testdata/info.json",
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/starcraft-test-2.0.0.tar.xz",
	approved: false,
	reason:   "", // unrecognized artifact
}}

func (s *storeSuite) TestStoreResolveAPIArtifactInspector(c *C) {
	for _, tc := range storeResolveAPIArtifactInspectorTests {
		a := metadata.NewArtifact()
		a.Metadata.Type = "application/json"
		a.Metadata.Size = 1234

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer f.Close()

		ins := store.NewStoreResolveAPIInspector(getTestStoreInspectorConfig())
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Check(a.Approved(), Equals, tc.approved, Commentf("%+v", tc))

		if tc.reason != "" {
			insp := a.ResponseInspection["store.resolve-api"]
			if tc.approved {
				c.Check(insp.Opinion, Equals, opinions.Approved)
			} else {
				c.Check(insp.Opinion, Equals, opinions.Rejected)
			}
			c.Check(insp.Reason, Equals, tc.reason)
			c.Check(insp.Annotations["resolved-craft-list"], DeepEquals, []string{"starcraft-test"})
			c.Check(insp.Annotations["resolved-package-list"], DeepEquals, []string{})
			c.Check(a.Metadata.Type, Equals, "application/x.canonical.store.resolve-api")
			c.Check(a.Metadata.Name, Equals, "Store protocol response")
			c.Check(a.Metadata.Size, Equals, int64(1234))
			c.Check(a.Metadata.Description, Equals, "Store response for resolve_revisions request")
		}
	}
}

func (s *storeSuite) TestStoreResolveAPIArtifactBadType(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "text/plain"
	a.Metadata.Size = 1743

	f, err := files.OpenArtifactFile("testdata/resolve.json")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := store.NewStoreInfoAPIInspector(getTestStoreInspectorConfig(), getTestBldbinInspectorConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Approved(), Equals, false)
}
