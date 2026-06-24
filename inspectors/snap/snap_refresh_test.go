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
	"io"
	"net/http"
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

func (s *snapSuite) TestInspectRefreshRequestAnnotatesRequestedChannel(c *C) {
	ins := snap.NewSnapRefreshInspector()
	a := metadata.NewArtifact()
	a.CurrentDownload = metadata.Download{URL: "https://api.snapcraft.io:443/v2/snaps/refresh"}
	a.Request = &http.Request{Body: io.NopCloser(strings.NewReader(`{"actions":[{"channel":"latest/edge"}]}`))}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)

	insp, ok := a.RequestInspection[ins.ID()]
	c.Assert(ok, Equals, true)
	c.Assert(insp.Opinion, Equals, opinions.Pending)
	c.Assert(insp.Annotations, DeepEquals, Annotation{
		"requested-channel": "latest/edge",
	})

	body, err := io.ReadAll(a.Request.Body)
	c.Assert(err, IsNil)
	c.Assert(string(body), Equals, `{"actions":[{"channel":"latest/edge"}]}`)
}

func (s *snapSuite) TestInspectRefreshRequestWithoutChannel(c *C) {
	ins := snap.NewSnapRefreshInspector()
	a := metadata.NewArtifact()
	a.CurrentDownload = metadata.Download{URL: "https://api.snapcraft.io:443/v2/snaps/refresh"}
	a.Request = &http.Request{Body: io.NopCloser(strings.NewReader(`{"context":[{"snap-id":"foo"}]}`))}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)

	insp, ok := a.RequestInspection[ins.ID()]
	c.Assert(ok, Equals, true)
	c.Assert(insp.Opinion, Equals, opinions.Pending)
	c.Assert(insp.Annotations, IsNil)
}

func (s *snapSuite) TestInspectRefreshRequestIgnoresInvalidJSON(c *C) {
	ins := snap.NewSnapRefreshInspector()
	a := metadata.NewArtifact()
	a.CurrentDownload = metadata.Download{URL: "https://api.snapcraft.io:443/v2/snaps/refresh"}
	a.Request = &http.Request{Body: io.NopCloser(strings.NewReader(`{`))}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)

	insp, ok := a.RequestInspection[ins.ID()]
	c.Assert(ok, Equals, true)
	c.Assert(insp.Opinion, Equals, opinions.Pending)
	c.Assert(insp.Annotations, IsNil)
}

func (s *snapSuite) TestSnapRefreshArtifactInspector(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/json"
	a.Metadata.Size = 3330

	f, err := files.OpenArtifactFile("testdata/refresh.json")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := snap.NewSnapRefreshInspector()
	a.SetRequestPending(ins, "test").Annotate(Annotation{"requested-channel": "latest/candidate"})
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, true)
	c.Check(a.Metadata.Type, Equals, "application/x.canonical.snap-refresh")
	c.Check(a.Metadata.Name, Equals, "Store protocol response")
	c.Check(a.Metadata.Size, Equals, int64(3330))
	c.Check(a.Metadata.ContentID, Equals, "stable:10660")
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

func (s *snapSuite) TestSnapRefreshInspectorAnnotatesMatchingSnapDownload(c *C) {
	ins := snap.NewSnapRefreshInspector()

	refreshReq := metadata.NewArtifact()
	refreshReq.CurrentDownload = metadata.Download{URL: "https://api.snapcraft.io:443/v2/snaps/refresh"}
	refreshReq.Request = &http.Request{Body: io.NopCloser(strings.NewReader(`{"actions":[{"channel":"latest/candidate"}]}`))}
	err := ins.InspectRequest(refreshReq)
	c.Assert(err, IsNil)

	refreshResp := metadata.NewArtifact()
	refreshResp.Metadata.Type = "application/json"
	refreshResp.Metadata.Size = 3330
	refreshResp.CurrentDownload = metadata.Download{URL: "https://api.snapcraft.io:443/v2/snaps/refresh"}
	refreshResp.RequestInspection = refreshReq.RequestInspection
	refreshFile, err := files.OpenArtifactFile("testdata/refresh.json")
	c.Assert(err, IsNil)
	defer refreshFile.Close()
	err = ins.InspectArtifact(refreshFile, refreshResp)
	c.Assert(err, IsNil)

	snapReq := metadata.NewArtifact()
	snapReq.CurrentDownload = metadata.Download{URL: "https://api.snapcraft.io:443/api/v1/snaps/download/Md1HBASHzP4i0bniScAjXGnOII9cEK6e_10660.snap"}
	err = ins.InspectRequest(snapReq)
	c.Assert(err, IsNil)
	insp, ok := snapReq.RequestInspection[ins.ID()]
	c.Assert(ok, Equals, true)
	c.Assert(insp.Opinion, Equals, opinions.Pending)
	c.Assert(insp.Annotations, DeepEquals, Annotation{
		"requested-channel": "latest/candidate",
		"effective-channel": "stable",
		"snap-id":           "Md1HBASHzP4i0bniScAjXGnOII9cEK6e",
		"revision":          10660,
		"name":              "go",
		"version":           "1.22.5",
	})

	snapResp := metadata.NewArtifact()
	snapResp.Metadata.Type = "application/x.squashfs"
	snapResp.CurrentDownload = snapReq.CurrentDownload
	snapResp.RequestInspection = snapReq.RequestInspection
	snapFile, err := files.OpenArtifactFile("testdata/UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7.snap")
	c.Assert(err, IsNil)
	defer snapFile.Close()
	err = ins.InspectArtifact(snapFile, snapResp)
	c.Assert(err, IsNil)

	respInsp, ok := snapResp.ResponseInspection[ins.ID()]
	c.Assert(ok, Equals, true)
	c.Assert(respInsp.Opinion, Equals, opinions.Unknown)
	c.Assert(respInsp.Annotations, DeepEquals, insp.Annotations)
	c.Check(snapResp.Metadata.ReqChannel, Equals, "")
	c.Check(snapResp.Metadata.Channel, Equals, "")
}

func (s *snapSuite) TestSnapRefreshInspectorIgnoresUnmatchedSnapDownload(c *C) {
	ins := snap.NewSnapRefreshInspector()
	a := metadata.NewArtifact()
	a.CurrentDownload = metadata.Download{URL: "https://api.snapcraft.io:443/api/v1/snaps/download/Md1HBASHzP4i0bniScAjXGnOII9cEK6e_10660.snap"}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	_, ok := a.RequestInspection[ins.ID()]
	c.Assert(ok, Equals, false)
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
