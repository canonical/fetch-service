// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2026 Canonical Ltd.
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
	c.Check(a.Metadata.ContentID, Equals, "stable:10660")
	c.Check(a.Metadata.Description, Equals, "Snap store response for refresh request")
	c.Check(a.ResponseInspection["snap.refresh"].Annotations, DeepEquals, Annotation{
		"refresh-results": []Annotation{{
			"tracking-channel":  "",
			"effective-channel": "stable",
			"release-timestamp": "2024-07-04T13:56:12.190670+00:00",
			"result":            "install",
			"snap-name":         "go",
			"snap-id":           "Md1HBASHzP4i0bniScAjXGnOII9cEK6e",
			"snap-revision":     10660,
			"snap-version":      "1.22.5",
			"architectures":     []string{"amd64"},
		}},
	})
}

func (s *snapSuite) TestSnapRefreshArtifactInspectorAddsTrackingChannels(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/json"
	a.Metadata.Size = 100

	refreshReq := `{"actions":[
		{"action":"refresh","instance-key":"k1","channel":"stable"},
		{"action":"refresh","snap-id":"snap-id-2","instance-key":"k2","channel":"latest/candidate"}
	]}`
	var err error
	a.Request, err = http.NewRequest("POST", "https://api.snapcraft.io:443/v2/snaps/refresh", io.NopCloser(strings.NewReader(refreshReq)))
	c.Assert(err, IsNil)

	ins := snap.NewSnapRefreshInspector()
	a.CurrentDownload.URL = "https://api.snapcraft.io:443/v2/snaps/refresh"
	err = ins.InspectRequest(a)
	c.Assert(err, IsNil)

	resp := strings.NewReader(`{
		"error-list": [],
		"results": [
			{
				"effective-channel": "latest/edge",
				"instance-key": "k2",
				"name": "snap-two",
				"released-at": "2026-06-28T04:05:06+00:00",
				"result": "refresh",
				"snap": {"architectures": ["amd64", "arm64"], "version": "4.5.6", "revision": 22},
				"snap-id": "snap-id-2"
			},
			{
				"effective-channel": "stable",
				"instance-key": "k1",
				"name": "snap-one",
				"released-at": "2026-06-28T01:02:03+00:00",
				"result": "refresh",
				"snap": {"architectures": ["amd64"], "version": "1.2.3", "revision": 11},
				"snap-id": "snap-id-1"
			}
		]
	}`)

	err = ins.InspectArtifact(resp, a)
	c.Assert(err, IsNil)

	ann := a.ResponseInspection["snap.refresh"].Annotations
	results, ok := ann["refresh-results"].([]Annotation)
	c.Assert(ok, Equals, true)
	c.Assert(results, HasLen, 2)
	c.Check(results[0]["tracking-channel"], Equals, "latest/candidate")
	c.Check(results[1]["tracking-channel"], Equals, "stable")
}

func (s *snapSuite) TestSnapRefreshArtifactInspectorTrackingChannel(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/json"
	a.Metadata.Size = 100

	refreshReq := `{"actions":[{"action":"refresh","instance-key":"k1","channel":"beta"}]}`
	var err error
	a.Request, err = http.NewRequest("POST", "https://api.snapcraft.io:443/v2/snaps/refresh", io.NopCloser(strings.NewReader(refreshReq)))
	c.Assert(err, IsNil)

	ins := snap.NewSnapRefreshInspector()
	a.CurrentDownload.URL = "https://api.snapcraft.io:443/v2/snaps/refresh"
	err = ins.InspectRequest(a)
	c.Assert(err, IsNil)

	resp := strings.NewReader(`{
		"error-list": [],
		"results": [
			{
				"effective-channel": "stable",
				"instance-key": "k1",
				"name": "snap-one",
				"released-at": "2026-06-28T01:02:03+00:00",
				"result": "refresh",
				"snap": {"architectures": ["amd64"], "version": "1.2.3", "revision": 11},
				"snap-id": "snap-id-1"
			}
		]
	}`)

	err = ins.InspectArtifact(resp, a)
	c.Assert(err, IsNil)

	ann := a.ResponseInspection["snap.refresh"].Annotations
	results, ok := ann["refresh-results"].([]Annotation)
	c.Assert(ok, Equals, true)
	c.Assert(results, HasLen, 1)
	c.Check(results[0]["tracking-channel"], Equals, "beta")
}

func (s *snapSuite) TestSnapRefreshArtifactInspectorUnexpectedInstanceKey(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/json"
	a.Metadata.Size = 100

	refreshReq := `{"actions":[{"action":"refresh","instance-key":"k1","snap-id":"snap-id-1","channel":"latest/stable"}]}`
	var err error
	a.Request, err = http.NewRequest("POST", "https://api.snapcraft.io:443/v2/snaps/refresh", io.NopCloser(strings.NewReader(refreshReq)))
	c.Assert(err, IsNil)

	ins := snap.NewSnapRefreshInspector()
	a.CurrentDownload.URL = "https://api.snapcraft.io:443/v2/snaps/refresh"
	err = ins.InspectRequest(a)
	c.Assert(err, IsNil)

	resp := strings.NewReader(`{
		"error-list": [],
		"results": [
			{
				"effective-channel": "stable",
				"instance-key": "unexpected-key",
				"name": "snap-one",
				"released-at": "2026-06-28T01:02:03+00:00",
				"result": "refresh",
				"snap": {"architectures": ["amd64"], "version": "1.2.3", "revision": 11},
				"snap-id": "snap-id-1"
			}
		]
	}`)

	err = ins.InspectArtifact(resp, a)
	c.Assert(err, IsNil)
	insp := a.ResponseInspection["snap.refresh"]
	c.Assert(insp, NotNil)
	c.Check(insp.Opinion, Equals, opinions.Rejected)
	c.Check(insp.Reason, Equals, "unexpected instance-key \"unexpected-key\" for refresh result")
}

func (s *snapSuite) TestSnapRefreshArtifactInspectorAllowsUnknownDownloadKey(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/json"
	a.Metadata.Size = 100

	refreshReq := `{"actions":[{"action":"refresh","instance-key":"k1","channel":"latest/stable"}]}`
	var err error
	a.Request, err = http.NewRequest("POST", "https://api.snapcraft.io:443/v2/snaps/refresh", io.NopCloser(strings.NewReader(refreshReq)))
	c.Assert(err, IsNil)

	ins := snap.NewSnapRefreshInspector()
	a.CurrentDownload.URL = "https://api.snapcraft.io:443/v2/snaps/refresh"
	err = ins.InspectRequest(a)
	c.Assert(err, IsNil)

	resp := strings.NewReader(`{
		"error-list": [],
		"results": [
			{
				"effective-channel": "stable",
				"instance-key": "k1",
				"name": "snap-one",
				"released-at": "2026-06-28T01:02:03+00:00",
				"result": "refresh",
				"snap": {"architectures": ["amd64"], "version": "1.2.3", "revision": 11},
				"snap-id": "snap-id-1"
			},
			{
				"effective-channel": "edge",
				"instance-key": "download-1",
				"name": "snap-two",
				"released-at": "2026-06-28T04:05:06+00:00",
				"result": "download",
				"snap": {"architectures": ["amd64"], "version": "4.5.6", "revision": 22},
				"snap-id": "snap-id-2"
			}
		]
	}`)

	err = ins.InspectArtifact(resp, a)
	c.Assert(err, IsNil)
	insp := a.ResponseInspection["snap.refresh"]
	c.Assert(insp, NotNil)
	c.Check(insp.Opinion, Equals, opinions.Approved)
}

func (s *snapSuite) TestSnapRefreshArtifactInspectorSnapIDMismatch(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/json"
	a.Metadata.Size = 100

	refreshReq := `{"actions":[
		{"action":"refresh","instance-key":"k1","snap-id":"wrong-snap-id","channel":"latest/candidate"},
		{"action":"refresh","instance-key":"k2","snap-id":"snap-id-2","channel":"latest/stable"}
	]}`
	var err error
	a.Request, err = http.NewRequest("POST", "https://api.snapcraft.io:443/v2/snaps/refresh", io.NopCloser(strings.NewReader(refreshReq)))
	c.Assert(err, IsNil)

	ins := snap.NewSnapRefreshInspector()
	a.CurrentDownload.URL = "https://api.snapcraft.io:443/v2/snaps/refresh"
	err = ins.InspectRequest(a)
	c.Assert(err, IsNil)

	resp := strings.NewReader(`{
		"error-list": [],
		"results": [
			{
				"effective-channel": "stable",
				"instance-key": "k1",
				"name": "snap-one",
				"released-at": "2026-06-28T01:02:03+00:00",
				"result": "refresh",
				"snap": {
					"architectures": ["amd64"],
					"version": "1.2.3",
					"revision": 11
				},
				"snap-id": "actual-snap-id"
			},
			{
				"effective-channel": "edge",
				"instance-key": "k2",
				"name": "snap-two",
				"released-at": "2026-06-28T04:05:06+00:00",
				"result": "refresh",
				"snap": {
					"architectures": ["amd64"],
					"version": "4.5.6",
					"revision": 22
				},
				"snap-id": "snap-id-2"
			}
		]
	}`)

	err = ins.InspectArtifact(resp, a)
	c.Assert(err, IsNil)
	insp := a.ResponseInspection["snap.refresh"]
	c.Assert(insp, NotNil)
	c.Check(insp.Opinion, Equals, opinions.Rejected)
	c.Check(insp.Reason, Equals, "refresh result snap-id does not match request action with instance-key \"k1\"")
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
