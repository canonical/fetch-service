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

package apt_test

import (
	"bytes"
	"os"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/apt"
	"github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type aptCommandsDetectorTest struct {
	filename string // The artifact file to test.
	result   bool   // Whether the artifact format is detected.
}

var aptCommandsDetectorTests = []aptCommandsDetectorTest{{
	filename: "testdata/Commands-amd64.xz",
	result:   true,
}, {
	filename: "testdata/Translation-zh_TW.xz",
	result:   false,
}, {
	filename: "testdata/2048.package",
	result:   false,
}}

func (s *aptSuite) TestAptCommandsID(c *C) {
	ins := apt.NewAptCommandsInspector(getTestAptConfig())
	c.Assert(ins.ID(), Equals, "apt.commands")
}

func (s *aptSuite) TestAptCommandsDetector(c *C) {
	for _, tc := range aptCommandsDetectorTests {
		data, err := os.ReadFile(tc.filename)
		c.Assert(err, IsNil)

		res := apt.AptCommandsDetector(data, uint32(len(data)))
		c.Check(res, Equals, tc.result, Commentf("test case: %+v", tc))
	}
}

type aptCommandsInspectRequestTest struct {
	url     string // The request URL.
	pending bool   // Whether the artifact is pending further inspection.
}

var aptCommandsInspectRequestTests = []aptCommandsInspectRequestTest{{
	// Correct commands file URL
	url:     "http://archive.ubuntu.com/ubuntu/dists/focal/main/cnf/by-hash/SHA256/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
	pending: true,
}, {
	// Translation file URL
	url:     "http://archive.ubuntu.com/ubuntu/dists/focal/main/i18n/by-hash/SHA256/4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed",
	pending: false,
}, {
	// Deb file URL
	url:     "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/by-hash/SHA256/6213291a10046e8188510a0ca41a75daedfb2922940f88888ee815694ab3e7b7",
	pending: false,
}, {
	// Other URL
	url:     "http://some.other.location/Commands-amd64.xz",
	pending: false,
}}

func (s *aptSuite) TestAptCommandsInspectRequest(c *C) {
	for _, tc := range aptCommandsInspectRequestTests {
		ins := apt.NewAptCommandsInspector(getAptInspectorConfig())
		a := metadata.NewArtifact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		if tc.pending {
			insp, ok := a.RequestInspection[ins.ID()]
			c.Assert(ok, Equals, true)
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		} else {
			insp, ok := a.RequestInspection[ins.ID()]
			if ok {
				c.Assert(insp.Opinion, Equals, opinions.Unknown)
			}
		}
	}
}

type aptCommandsInspectorTest struct {
	filename string // The artifact file to test.
	result   bool   // Whether this file is approved by the inspector,
	reason   string // The reason for approval or rejection.
	suite    string // The suite, in case of approval.
	count    int    // The number of packages processed, in case of approval.
}

var aptCommandsInspectorTests = []aptCommandsInspectorTest{{
	filename: "testdata/Commands-amd64.xz",
	result:   true,
	reason:   "commands file successfully parsed",
	suite:    "jammy-security",
	count:    8,
}, {
	filename: "testdata/Packages.xz",
	result:   false,
	reason:   "ill-formed commands file header",
	suite:    "",
	count:    0,
}, {
	filename: "testdata/2048.package",
	result:   false,
	reason:   "cannot read xz file",
	suite:    "",
	count:    0,
}}

func (s *aptSuite) TestAptCommandsInspector(c *C) {

	for _, tc := range aptCommandsInspectorTests {
		commandsArtifactFile, _ := os.Open(tc.filename)
		commandsArtifactData := make([]byte, 1024*128)
		_, err := commandsArtifactFile.Read(commandsArtifactData)
		c.Assert(err, IsNil)

		ins := apt.NewAptCommandsInspector(getTestAptConfig())

		a := metadata.NewArtifact()
		a.SetRequestPending(ins, "test")
		a.CurrentDownload.URL = "http://archive.ubuntu.com/ubuntu/dists/devel/main/cnf/by-hash/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf"
		a.Metadata.Type = "application/x.apt.commands"
		a.Metadata.Sha256, _ = digests.NewSha256Digest("6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf")
		a.Metadata.Size = 4343
		a.ResponseInspection["apt.release"] = &common.Inspection{Annotations: common.Annotation{"vendor": "somevendor"}}

		f := bytes.NewReader(commandsArtifactData)
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		c.Assert(a.Approved(), Equals, tc.result)
		c.Assert(a.ResponseInspection[ins.ID()].Reason, Equals, tc.reason)

		if tc.result {
			c.Assert(a.ResponseInspection[ins.ID()].Annotations["suite"], Equals, tc.suite)
			c.Assert(a.ResponseInspection[ins.ID()].Annotations["count"], Equals, tc.count)
			c.Assert(a.Metadata.Type, Equals, "application/x.apt.commands")
			c.Assert(a.Metadata.Name, Equals, "Commands")
			c.Assert(a.Metadata.Sha256.String(), Equals, "6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf")
			c.Assert(a.Metadata.Size, Equals, int64(4343))
			c.Assert(a.Metadata.Vendor, Equals, "somevendor")
			c.Assert(a.Metadata.Author, Equals, "somevendor")
		}
	}
}
