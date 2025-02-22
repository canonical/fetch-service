// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2025 Canonical Ltd.
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

type aptTranslationDetectorTest struct {
	filename string // The path to the test file
	detected bool   // The expected detection result
}

var aptTranslationDetectorTests = []aptTranslationDetectorTest{{
	filename: "testdata/Translation-zh_TW.xz",
	detected: true,
}, {
	filename: "testdata/Translation-zh_TW-bad.xz",
	detected: false,
}, {
	filename: "testdata/2048.package",
	detected: false,
}}

func (s *aptSuite) TestAptTranslationDetector(c *C) {
	for _, tc := range aptTranslationDetectorTests {
		data, err := os.ReadFile(tc.filename)
		c.Assert(err, IsNil)

		res := apt.AptTranslationDetector(data, uint32(len(data)))
		c.Check(res, Equals, tc.detected, Commentf("test case: %+v", tc))
	}
}

type aptTranslationInspectRequestTest struct {
	url     string // The artifact request URL
	pending bool   // Whether the artifact is expected to be set as pending
}

var aptTranslationInspectRequestTests = []aptTranslationInspectRequestTest{{
	url:     "http://archive.ubuntu.com/ubuntu/dists/focal/main/i18n/by-hash/SHA256/4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed",
	pending: true,
}, {
	url:     "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/by-hash/SHA256/6213291a10046e8188510a0ca41a75daedfb2922940f88888ee815694ab3e7b7",
	pending: false,
}, {
	url:     "http://some.other.location/Translation-zh_TW.xz",
	pending: false,
}}

func (s *aptSuite) TestAptTranslationInspectRequest(c *C) {
	for _, tc := range aptTranslationInspectRequestTests {
		ins := apt.NewAptTranslationInspector(getAptInspectorConfig())
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

type aptTranslationInspectArtifactTest struct {
	filename string // The path to the file to be inspected
	approved bool   // Whether the artifact is expected to be approved
	reason   string // The reason for approval or rejection
	lang     string // The expected translation file language
	entries  int    // The expected number of translation entries
}

var aptTranslationInspectArtifactTests = []aptTranslationInspectArtifactTest{{
	filename: "testdata/Translation-zh_TW.xz",
	approved: true,
	reason:   "translation file successfully parsed",
	lang:     "zh_TW",
	entries:  3,
}, {
	filename: "testdata/Translation-zh_TW-bad.xz",
	approved: false,
	reason:   "not a valid translation file",
	lang:     "",
	entries:  0,
}, {
	filename: "testdata/2048.package",
	approved: false,
	reason:   "cannot read xz file",
	lang:     "",
	entries:  0,
}}

func (s *aptSuite) TestAptTranslationInspectArtifact(c *C) {
	for _, tc := range aptTranslationInspectArtifactTests {
		data, err := os.ReadFile(tc.filename)
		c.Assert(err, IsNil)

		ins := apt.NewAptTranslationInspector(getTestAptConfig())

		a := metadata.NewArtifact()
		a.SetRequestPending(ins, "test")
		a.CurrentDownload.URL = "http://archive.ubuntu.com/ubuntu/dists/devel/main/i18n/by-hash/SHA256/4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed"
		a.Metadata.Type = "application/x.apt.translation"
		a.Metadata.Sha256, _ = digests.NewSha256Digest("4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed")
		a.Metadata.Size = 4242
		a.ResponseInspection["apt.release"] = &common.Inspection{Annotations: common.Annotation{"vendor": "somevendor"}}

		f := bytes.NewReader(data)
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		c.Assert(a.Approved(), Equals, tc.approved)
		c.Assert(a.ResponseInspection[ins.ID()].Reason, Equals, tc.reason)

		if tc.approved {
			c.Assert(a.ResponseInspection[ins.ID()].Annotations["translation-language"], Equals, tc.lang)
			c.Assert(a.ResponseInspection[ins.ID()].Annotations["translation-count"], Equals, tc.entries)
			c.Assert(a.Metadata.Type, Equals, "application/x.apt.translation")
			c.Assert(a.Metadata.Name, Equals, "Translation")
			c.Assert(a.Metadata.Sha256.String(), Equals, "4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed")
			c.Assert(a.Metadata.Size, Equals, int64(4242))
			c.Assert(a.Metadata.Vendor, Equals, "somevendor")
			c.Assert(a.Metadata.Author, Equals, "somevendor")
		}

	}
}
