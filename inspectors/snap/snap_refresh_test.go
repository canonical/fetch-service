// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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
	"strings"

	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/snap"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

func (s *snapSuite) TestSnapRefreshInspectorID(c *C) {
	ins := snap.NewSnapRefreshInspector()
	c.Assert(ins.ID(), Equals, "snap.refresh")
}

type inspectRefreshRequestTest struct {
	url     string // The refresh request URL
	pending bool   // Whether the inspection should be set to pending
}

var inspectRefreshRequestTests = []inspectRefreshRequestTest{{
	url:     "https://api.snapcraft.io:443/v2/snaps/refresh",
	pending: true,
}, {
	url:     "https://api.snapcraft.io:443/v1/snaps/refresh",
	pending: false,
}, {
	url:     "https://api.snapcraft.io:443/v2/snaps/info",
	pending: false,
}, {
	url:     "http://api.snapcraft.io/v2/snaps/refresh",
	pending: false,
}}

func (s *snapSuite) TestInspectRefreshRequest(c *C) {
	for _, tc := range inspectRefreshRequestTests {
		ins := snap.NewSnapRefreshInspector()
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

func (s *snapSuite) TestSnapRefreshArtifactInspector(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/json"
	a.Metadata.Size = 3330

	f, err := files.OpenArtifactFile("testdata/refresh.json")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := snap.NewSnapRefreshInspector()
	a.SetRequestPending(ins, "test")
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, true)
	c.Check(a.Metadata.Type, Equals, "application/x.canonical.snap-refresh")
	c.Check(a.Metadata.Name, Equals, "Store protocol response")
	c.Check(a.Metadata.Size, Equals, int64(3330))
	c.Check(a.Metadata.Description, Equals, "Snap store response for refresh request")
	c.Check(a.ResponseInspection["snap.refresh"].Annotations, DeepEquals, Annotation{
		"name":     "go",
		"version":  "1.22.5",
		"revision": 10660,
		"channel":  "stable",
		"result":   "install",
		"snap-id":  "Md1HBASHzP4i0bniScAjXGnOII9cEK6e",
	})
}

func (s *snapSuite) TestSnapRefreshArtifactBadType(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "text/plain"
	a.Metadata.Size = 3330

	f, err := files.OpenArtifactFile("testdata/refresh.json")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := snap.NewSnapRefreshInspector()
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
}

func (s *snapSuite) TestSnapRefreshArtifactBadContent(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "text/plain"
	a.Metadata.Size = 3330

	f := strings.NewReader(`{"content": "bad"}`)

	ins := snap.NewSnapRefreshInspector()
	err := ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
}
