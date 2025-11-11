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

func (s *snapSuite) TestSnapAuthNonceInspectorID(c *C) {
	ins := snap.NewSnapAuthNonceInspector()
	c.Assert(ins.ID(), Equals, "snap.auth-nonce")
}

type snapAuthNonceInspectRequestTest struct {
	url     string // The info request URL
	pending bool   // Whether the inspection result should be pending
}

var snapAuthNonceInspectRequestTests = []snapAuthNonceInspectRequestTest{{
	url:     "https://api.snapcraft.io:443/api/v1/snaps/auth/nonces",
	pending: true,
}, {
	url:     "https://api.snapcraft.io:443/api/v1/snaps/auth/nonces2",
	pending: false,
}, {
	url:     "http://api.snapcraft.io/api/v1/snaps/auth/nonces2",
	pending: false,
}, {
	url:     "https://api.snapcraft.io:443/v2/auth/nonces",
	pending: false,
}}

func (s *snapSuite) TestSnapAuthNonceInspectRequest(c *C) {
	for _, tc := range snapAuthNonceInspectRequestTests {
		ins := snap.NewSnapAuthNonceInspector()
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

type snapAuthNonceArtifactInspectorTest struct {
	filename string // The test file name
	pending  bool   // Whether the request was set to pending
	approved bool   // Whether the artifact should be approved
	reason   string // The reason for approval or rejection
}

var snapAuthNonceArtifactInspectorTests = []snapAuthNonceArtifactInspectorTest{{
	filename: "testdata/nonce.json",
	pending:  true,
	approved: true,
	reason:   "valid format for snapd authentication nonce",
}, {
	filename: "testdata/nonce.json",
	pending:  false,
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/nonce-extra.json",
	pending:  false,
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/refresh.json",
	pending:  false,
	approved: false,
	reason:   "", // unrecognized artifact
}}

func (s *snapSuite) TestSnapAuthNonceArtifactInspector(c *C) {
	for _, tc := range snapAuthNonceArtifactInspectorTests {
		a := metadata.NewArtifact()
		a.Metadata.Type = "application/json"
		a.Metadata.Size = 1234

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer f.Close()

		ins := snap.NewSnapAuthNonceInspector()
		if tc.pending {
			a.SetRequestPending(ins, "test")
		}
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Check(a.Approved(), Equals, tc.approved, Commentf("%+v", tc))

		if tc.approved {
			insp := a.ResponseInspection["snap.auth-nonce"]
			c.Assert(insp, Not(IsNil))
			c.Check(insp.Opinion, Equals, opinions.Approved)

			c.Check(insp.Reason, Equals, tc.reason)
			c.Check(a.Metadata.Type, Equals, "application/x.canonical.snapd-auth-nonce")
			c.Check(a.Metadata.Name, Equals, "Authentication nonce")
			c.Check(a.Metadata.Description, Equals, "Snapd authentication nonce")
		}
	}
}

func (s *snapSuite) TestSnapAuthNonceArtifactBadType(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "text/plain"
	a.Metadata.Size = 1743

	f, err := files.OpenArtifactFile("testdata/nonce.json")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := snap.NewSnapAuthNonceInspector()
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Approved(), Equals, false)
}
