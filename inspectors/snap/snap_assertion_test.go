// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024 Canonical Ltd.
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

func (s *snapSuite) TestSnapAssertionDetector(c *C) {
	for _, tc := range []struct {
		content string
		result  bool
	}{
		{"", false},
		{"type: my-assertion-type\nauthority-id: the-authority-id and some more filler data but staying at just two lines which is not enough for this to work\n", false},
		{"type: my-assertion-type\nauthority-id: the-authority-id\nsome more filler data to get to the miminum size we consider valid for an assertion file\n", true},
	} {
		res := snap.AssertionDetector([]byte(tc.content), 1024)
		c.Assert(res, Equals, tc.result)
	}
}

func (s *snapSuite) TestInspectAssertionRequest(c *C) {
	for _, tc := range []struct {
		url       string
		hasAccept bool
		approved  bool
	}{
		{"https://api.snapcraft.io:443/v2/assertions/snap-revision/", true, true},
		{"https://api.snapcraft.io:443/v2/assertions/snap-declaration/", true, true},
		{"https://api.snapcraft.io:443/v2/assertions/account/", true, true},
		{"https://api.snapcraft.io:443/v2/assertions/account-key/", true, true},
		{"https://api.snapcraft.io:443/v2/assertions/snap-revision/", false, false},
		{"https://api.snapcraft.io:443/v1/assertions/snap-revision/", true, false},
		{"https://api.snapcraft.io:443/v3/assertions/snap-revision/", true, false},
		{"https://api.snapcraft.io:443/v2/assertions/snap-revision", true, false},
		{"https://api.snapcraft.io:443/v2/assertions/something-else/", true, false},
		{"https://api.snapcraft.io:443/v2/assertions/", true, false},
		{"http://api.snapcraft.io/v2/assertions/snap-revision/", true, false},
	} {
		ins := snap.NewSnapAssertionInspector()
		a := metadata.NewArtefact()
		a.CurrentDownload = metadata.Download{URL: tc.url}
		if tc.hasAccept {
			a.CurrentDownload.RequestHeader = map[string][]string{"Accept": []string{"application/x.ubuntu.assertion"}}
		}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, tc.approved, Commentf("test case: %+v", tc))
		if tc.approved {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		}
	}
}

func (s *snapSuite) TestSnapAssertionArtefactInspector(c *C) {
	for _, tc := range []struct {
		testfile  string
		rejection string
		filetype  string
	}{
		{"testdata/snap-revision.assert", "", "application/x.ubuntu.assertion.snap-revision"},
		{"testdata/snap-declaration.assert", "", "application/x.ubuntu.assertion.snap-declaration"},
		{"testdata/account.assert", "", "application/x.ubuntu.assertion.account"},
		{"testdata/account-key.assert", "", "application/x.ubuntu.assertion.account-key"},
		{"testdata/bad-assertion.assert", "error parsing assertion", ""},
		{"testdata/bad-signature.assert", "assertion signature verification failed", ""},
	} {
		a := metadata.NewArtefact()
		a.Metadata.Type = "application/x.ubuntu.assertion"
		a.Metadata.Size = 3330
		a.MimeType = mimetype.Lookup("application/x.ubuntu.assertion")
		a.CurrentDownload.ContentType = "application/x.ubuntu.assertion"

		f, err := files.OpenArtefactFile(tc.testfile)
		c.Assert(err, IsNil)
		defer f.Close()

		ins := snap.NewSnapAssertionInspector()
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtefact(f, a)
		c.Assert(err, IsNil)
		c.Assert(a.Approved(), Equals, tc.rejection == "", Commentf("test case: %+v", tc))

		if tc.rejection == "" {
			c.Check(a.Metadata.Type, Equals, tc.filetype)
			c.Check(a.Metadata.Name, Equals, "assertion")
			c.Check(a.Metadata.Size, Equals, int64(3330))
			c.Check(a.Metadata.Vendor, Equals, "canonical")
			c.Check(a.Metadata.Author, Equals, "canonical")
		} else {
			c.Check(a.ResponseInspection["snap.assertion"].Reason, Equals, tc.rejection)
		}
	}
}

func (s *snapSuite) TestSnapAssertionArtefactBadType(c *C) {
	a := metadata.NewArtefact()
	a.Metadata.Type = "application/x.ubuntu.assertion"
	a.Metadata.Size = 3330

	f, err := files.OpenArtefactFile("testdata/refresh.json")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := snap.NewSnapAssertionInspector()
	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
}

func (s *snapSuite) TestSnapAssertionArtefactBadContent(c *C) {
	a := metadata.NewArtefact()
	a.Metadata.Type = "text/plain"
	a.Metadata.Size = 3330

	f := strings.NewReader(`{"content": "bad"}`)

	ins := snap.NewSnapAssertionInspector()
	err := ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
}
