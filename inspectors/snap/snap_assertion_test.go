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

	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/snap"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

func (s *snapSuite) TestSnapAssertionInspectorID(c *C) {
	ins := snap.NewSnapAssertionInspector()
	c.Assert(ins.ID(), Equals, "snap.assertion")
}

type snapAssertionDetectorTest struct {
	content  string // The assertion payload
	detected bool   // Whether the content should be detected as an assertion
}

var snapAssertionDetectorTests = []snapAssertionDetectorTest{{
	content:  "",
	detected: false,
}, {
	content:  "type: my-assertion-type\nauthority-id: the-authority-id and some more filler data but staying at just two lines which is not enough for this to work\n",
	detected: false,
}, {
	content:  "type: my-assertion-type\nformat: 1\nsome more filler data to get to the miminum size we consider valid for an assertion file, and then more data.\n",
	detected: false,
}, {
	content:  "type: my-assertion-type\nformat: 1\nmention authority-id and some more filler data to get to the miminum size we consider valid for an assertion file\n",
	detected: false,
}, {
	content:  "type: my-assertion-type\nauthority-id: the-authority-id\nsome more filler data to get to the miminum size we consider valid for an assertion file\n",
	detected: true,
}, {
	content:  "type: my-assertion-type\nformat: 1\nauthority-id: the-authority-id\nsome more filler data to get to the miminum size we consider valid for an assertion file\n",
	detected: true,
}}

func (s *snapSuite) TestSnapAssertionDetector(c *C) {
	for _, tc := range snapAssertionDetectorTests {
		res := snap.AssertionDetector([]byte(tc.content), 1024)
		c.Assert(res, Equals, tc.detected, Commentf("test case: %+v", tc))
	}
}

type snapAssertionInspectRequestTest struct {
	url     string // The request URL
	pending bool   // The expected inspection result
	reason  string // The reason for the inspection result
}

var snapAssertionInspectRequestTests = []snapAssertionInspectRequestTest{{
	url:     "https://api.snapcraft.io:443/v2/assertions/snap-revision/",
	pending: true,
	reason:  "valid URL for snap-revision assertion download",
}, {
	url:     "https://api.snapcraft.io:443/v2/assertions/snap-declaration/",
	pending: true,
	reason:  "valid URL for snap-declaration assertion download",
}, {
	url:     "https://api.snapcraft.io:443/v2/assertions/account/",
	pending: true,
	reason:  "valid URL for account-key assertion download",
}, {
	url:     "https://api.snapcraft.io:443/v2/assertions/account-key/",
	pending: true,
	reason:  "valid URL for account-key assertion download",
}, {
	url:     "https://api.snapcraft.io:443/api/v1/snaps/auth/devices/",
	pending: true,
	reason:  "valid URL for serial assertion download",
}, {
	url:     "https://api.snapcraft.io:443/api/v1/snaps/auth/devices",
	pending: true,
	reason:  "valid URL for serial assertion download",
}, {
	url:     "https://api.snapcraft.io:443/v2/assertions/snap-revision",
	pending: false,
}, {
	url:     "https://api.snapcraft.io:443/v1/assertions/snap-revision/",
	pending: false,
}, {
	url:     "https://api.snapcraft.io:443/v2/assertions/something-else/",
	pending: false,
}, {
	url:     "https://api.snapcraft.io:443/v2/assertions/",
	pending: false,
}, {
	url:     "http://api.snapcraft.io/v2/assertions/snap-revision/",
	pending: false,
}}

func (s *snapSuite) TestSnapAssertionInspectRequest(c *C) {
	for _, tc := range snapAssertionInspectRequestTests {
		ins := snap.NewSnapAssertionInspector()
		a := metadata.NewArtifact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, tc.pending, Commentf("test case: %+v", tc))
		if ok {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
			c.Assert(insp.Reason, Equals, tc.reason)
		}
	}
}

type snapAssertionArtifactInspectorTest struct {
	filename string // The path to the artifact to be tested
	approved bool   // Whether this artifact is expected to be approved
	reason   string // The reason for approval or rejection
	filetype string // The expected file type
}

var snapAssertionArtifactInspectorTests = []snapAssertionArtifactInspectorTest{{
	filename: "testdata/snap-revision.assert",
	approved: true,
	reason:   "valid snap assertion",
	filetype: "application/x.ubuntu.assertion.snap-revision",
}, {
	filename: "testdata/snap-declaration.assert",
	approved: true,
	reason:   "valid snap assertion",
	filetype: "application/x.ubuntu.assertion.snap-declaration",
}, {
	filename: "testdata/snap-declaration-2.assert",
	approved: true,
	reason:   "valid snap assertion",
	filetype: "application/x.ubuntu.assertion.snap-declaration",
}, {
	filename: "testdata/account.assert",
	approved: true,
	reason:   "valid snap assertion",
	filetype: "application/x.ubuntu.assertion.account",
}, {
	filename: "testdata/account-key.assert",
	approved: true,
	reason:   "valid snap assertion",
	filetype: "application/x.ubuntu.assertion.account-key",
}, {
	filename: "testdata/serial.assert",
	approved: true,
	reason:   "valid snap assertion",
	filetype: "application/x.ubuntu.assertion.serial",
}, {
	filename: "testdata/bad-assertion.assert",
	approved: false,
	reason:   "error parsing assertion",
	filetype: "application/x.ubuntu.assertion",
}, {
	filename: "testdata/bad-signature.assert",
	approved: false,
	reason:   "assertion signature verification failed",
	filetype: "application/x.ubuntu.assertion.snap-revision",
}}

func (s *snapSuite) TestSnapAssertionArtifactInspector(c *C) {
	for _, tc := range snapAssertionArtifactInspectorTests {
		a := metadata.NewArtifact()
		a.Metadata.Type = "application/x.ubuntu.assertion"
		a.Metadata.Size = 3330
		a.MimeType = mimetype.Lookup("application/x.ubuntu.assertion")
		a.CurrentDownload.ContentType = "application/x.ubuntu.assertion"

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer f.Close()

		ins := snap.NewSnapAssertionInspector()
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Assert(a.Approved(), Equals, tc.approved, Commentf("test case: %+v", tc))
		c.Check(a.ResponseInspection["snap.assertion"].Reason, Equals, tc.reason)
		c.Check(a.Metadata.Type, Equals, tc.filetype)

		if tc.approved {
			c.Check(a.Metadata.Name, Equals, "assertion")
			c.Check(a.Metadata.Size, Equals, int64(3330))
			c.Check(a.Metadata.Vendor, Equals, "canonical")
			c.Check(a.Metadata.Author, Equals, "canonical")
		}
	}
}

func (s *snapSuite) TestSnapAssertionArtifactBadType(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/x.ubuntu.assertion"
	a.Metadata.Size = 3330

	f, err := files.OpenArtifactFile("testdata/refresh.json")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := snap.NewSnapAssertionInspector()
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
}

func (s *snapSuite) TestSnapAssertionArtifactBadContent(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "text/plain"
	a.Metadata.Size = 3330

	f := strings.NewReader(`{"content": "bad"}`)

	ins := snap.NewSnapAssertionInspector()
	err := ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
}
