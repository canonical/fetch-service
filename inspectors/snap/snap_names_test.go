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

package snap_test

import (
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/snap"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

func (s *snapSuite) TestSnapNamesInspectorID(c *C) {
	ins := snap.NewSnapNamesInspector()
	c.Assert(ins.ID(), Equals, "snap.names")
}

type snapNamesInspectRequestTest struct {
	url     string // The info request URL
	pending bool   // Whether the inspection result should be pending
}

var snapNamesInspectRequestTests = []snapNamesInspectRequestTest{{
	url:     "https://api.snapcraft.io:443/api/v1/snaps/names",
	pending: true,
}, {
	url:     "https://api.snapcraft.io:443/api/v1/snaps/names?confinement=strict%2Cclassic",
	pending: true,
}, {
	url:     "https://api.snapcraft.io:443/api/v1/snaps/names2",
	pending: false,
}, {
	url:     "http://api.snapcraft.io/api/v1/snaps/names",
	pending: false,
}}

func (s *snapSuite) TestSnapNamesInspectRequest(c *C) {
	for _, tc := range snapNamesInspectRequestTests {
		ins := snap.NewSnapNamesInspector()
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

type snapNamesArtifactInspectorTest struct {
	filename string // The test file name
	pending  bool   // Whether the request was set to pending
	approved bool   // Whether the artifact should be approved
	reason   string // The reason for approval or rejection
}

var snapNamesArtifactInspectorTests = []snapNamesArtifactInspectorTest{{
	filename: "testdata/names.json",
	pending:  true,
	approved: true,
	reason:   "valid Snap package names list",
}, {
	filename: "testdata/names.json",
	pending:  false,
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/sections.json",
	pending:  false,
	approved: false,
	reason:   "", // unrecognized artifact
}}

func (s *snapSuite) TestSnapNamesArtifactInspector(c *C) {
	for _, tc := range snapNamesArtifactInspectorTests {
		a := metadata.NewArtifact()
		a.Metadata.Type = "application/json"
		a.Metadata.Size = 1234

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		ins := snap.NewSnapNamesInspector()
		if tc.pending {
			a.SetRequestPending(ins, "test")
		}
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Check(a.Approved(), Equals, tc.approved, Commentf("test case: %+v", tc))

		if tc.approved {
			insp := a.ResponseInspection["snap.names"]
			c.Assert(insp, Not(IsNil))
			c.Check(insp.Opinion, Equals, opinions.Approved)

			c.Check(insp.Reason, Equals, tc.reason)
			c.Check(insp.Annotations["entries"], Equals, 7)
			c.Check(a.Metadata.Type, Equals, "application/x.canonical.snap-names")
			c.Check(a.Metadata.Name, Equals, "Snap names list")
			c.Check(a.Metadata.Description, Equals, "List of Snap package names")
		}
	}
}

func (s *snapSuite) TestSnapNamesArtifactBadType(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "text/plain"
	a.Metadata.Size = 1743

	f, err := files.OpenArtifactFile("testdata/names.json")
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	ins := snap.NewSnapNamesInspector()
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Approved(), Equals, false)
}
