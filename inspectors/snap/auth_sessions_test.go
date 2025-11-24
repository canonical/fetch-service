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

func (s *snapSuite) TestSnapAuthSessionsInspectorID(c *C) {
	ins := snap.NewSnapAuthSessionsInspector()
	c.Assert(ins.ID(), Equals, "snap.auth-sessions")
}

type snapAuthSessionsInspectRequestTest struct {
	url     string // The info request URL
	pending bool   // Whether the inspection result should be pending
}

var snapAuthSessionsInspectRequestTests = []snapAuthSessionsInspectRequestTest{{
	url:     "https://api.snapcraft.io:443/api/v1/snaps/auth/sessions",
	pending: true,
}, {
	url:     "https://api.snapcraft.io:443/api/v1/snaps/auth/sessions2",
	pending: false,
}, {
	url:     "http://api.snapcraft.io/api/v1/snaps/auth/sessions",
	pending: false,
}, {
	url:     "https://api.snapcraft.io:443/v2/auth/sessions",
	pending: false,
}}

func (s *snapSuite) TestSnapAuthSessionsInspectRequest(c *C) {
	for _, tc := range snapAuthSessionsInspectRequestTests {
		ins := snap.NewSnapAuthSessionsInspector()
		a := metadata.NewArtifact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		id := ins.ID()
		insp, ok := a.RequestInspection[id]
		c.Assert(ok, Equals, tc.pending, Commentf("test case: %+v", tc))
		if tc.pending {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		}
	}
}

type snapAuthSessionsArtifactInspectorTest struct {
	filename string // The test file name
	pending  bool   // Whether the request was set to pending
	approved bool   // Whether the artifact should be approved
	reason   string // The reason for approval or rejection
}

var snapAuthSessionsArtifactInspectorTests = []snapAuthSessionsArtifactInspectorTest{{
	filename: "testdata/sessions.json",
	pending:  true,
	approved: true,
	reason:   "valid format for snapd session authentication",
}, {
	filename: "testdata/sessions.json",
	pending:  false,
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/sessions-extra.json",
	pending:  false,
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/refresh.json",
	pending:  false,
	approved: false,
	reason:   "", // unrecognized artifact
}}

func (s *snapSuite) TestSnapAuthSessionsArtifactInspector(c *C) {
	for _, tc := range snapAuthSessionsArtifactInspectorTests {
		a := metadata.NewArtifact()
		a.Metadata.Type = "application/json"
		a.Metadata.Size = 1234

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer f.Close()

		ins := snap.NewSnapAuthSessionsInspector()
		if tc.pending {
			a.SetRequestPending(ins, "test")
		}
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Check(a.Approved(), Equals, tc.approved, Commentf("%+v", tc))

		if tc.approved {
			insp := a.ResponseInspection["snap.auth-sessions"]
			c.Assert(insp, Not(IsNil), Commentf("test case: %+v", tc))
			c.Check(insp.Opinion, Equals, opinions.Approved)

			c.Check(insp.Reason, Equals, tc.reason)
			c.Check(a.Metadata.Type, Equals, "application/x.canonical.snapd-auth-sessions")
			c.Check(a.Metadata.Name, Equals, "Session authentication")
			c.Check(a.Metadata.Description, Equals, "Snapd session authentication")
		}
	}
}

func (s *snapSuite) TestSnapAuthSessionsArtifactBadType(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "text/plain"
	a.Metadata.Size = 1743

	f, err := files.OpenArtifactFile("testdata/sessions.json")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := snap.NewSnapAuthSessionsInspector()
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Approved(), Equals, false)
}
