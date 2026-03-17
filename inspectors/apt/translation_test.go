// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2026 Canonical Ltd.
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
	"io"
	"os"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/apt"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type aptTranslationDetectorTest struct {
	filename string // The file to test
	result   bool   // Expected result
}

var aptTranslationDetectorTests = []aptTranslationDetectorTest{{
	filename: "testdata/Translation-en.xz",
	result:   true,
}, {
	filename: "testdata/Translation-zh_TW.xz",
	result:   true,
}, {
	filename: "testdata/Translation-zh_TW-bad.xz",
	result:   false,
}, {
	filename: "testdata/2048.package",
	result:   false,
}}

func (s *aptSuite) TestAptTranslationDetector(c *C) {
	for _, tc := range aptTranslationDetectorTests {
		data, err := os.ReadFile(tc.filename)
		c.Assert(err, IsNil)

		res := apt.AptTranslationDetector(data, uint32(len(data)))
		c.Check(res, Equals, tc.result, Commentf("test case: %+v", tc))
	}
}

type aptTranslationInspectRequestTest struct {
	url     string // The request URL
	pending bool   // Expected result
}

var aptTranslationInspectRequestTests = []aptTranslationInspectRequestTest{{
	url:     "http://archive.ubuntu.com/ubuntu/dists/focal/main/i18n/by-hash/SHA256/4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed",
	pending: true,
}, {
	url:     "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/by-hash/SHA256/6213291a10046e8188510a0ca41a75daedfb2922940f88888ee815694ab3e7b7",
	pending: false,
}, {
	url:     "http://security.ubuntu.com/ubuntu/dists/noble-security/main/i18n/Translation-en",
	pending: true,
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

type aptTranslationInspectorTest struct {
	dataFile string // The file to test
	approved bool   // Expected result
	reason   string // Reason for opinion
	language string // Expected translation language
	count    int    // Expected translation count
}

var aptTranslationInspectorTests = []aptTranslationInspectorTest{{
	dataFile: "testdata/Translation-en.xz",
	approved: true,
	reason:   "translation file successfully parsed",
	language: "en",
	count:    1,
}, {
	dataFile: "testdata/Translation-zh_TW.xz",
	approved: true,
	reason:   "translation file successfully parsed",
	language: "zh_TW",
	count:    3,
}, {
	dataFile: "testdata/Translation-zh_TW-bad.xz",
	approved: false,
	reason:   "not a valid translation file",
}, {
	dataFile: "testdata/2048.package",
	approved: false,
	reason:   "cannot read xz file",
}}

func (s *aptSuite) TestAptTranslationInspector(c *C) {
	for _, tc := range aptTranslationInspectorTests {
		translationArtifactData, err := os.ReadFile(tc.dataFile)
		c.Assert(err, IsNil, Commentf("test case: %+v\n", tc))

		ins := apt.NewAptTranslationInspector(getTestAptConfig())

		a := metadata.NewArtifact()
		a.SetRequestPending(ins, "test").Annotate(Annotation{"suite": "jammy"})
		a.CurrentDownload.URL = "http://archive.ubuntu.com/ubuntu/dists/devel/main/i18n/by-hash/SHA256/4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed"
		a.Metadata.Type = "application/x.apt.translation"
		a.Metadata.Sha256, _ = digests.NewSha256Digest("4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed")
		a.Metadata.Size = 4242
		a.ResponseInspection["apt.release"] = &Inspection{Annotations: Annotation{
			"release-file": "release-file-digest",
			"vendor":       "somevendor",
		}}

		f := bytes.NewReader(translationArtifactData)
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		c.Assert(a.Approved(), Equals, tc.approved)
		c.Assert(a.ResponseInspection[ins.ID()].Reason, Equals, tc.reason)

		if tc.approved {
			c.Assert(a.ResponseInspection[ins.ID()].Annotations["translation-language"], Equals, tc.language)
			c.Assert(a.ResponseInspection[ins.ID()].Annotations["translation-count"], Equals, tc.count)
			c.Assert(a.Metadata.Type, Equals, "application/x.apt.translation")
			c.Assert(a.Metadata.Name, Equals, "Translation")
			c.Assert(a.Metadata.Sha256.String(), Equals, "4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed")
			c.Assert(a.Metadata.Size, Equals, int64(4242))
			c.Assert(a.Metadata.Vendor, Equals, "somevendor")
			c.Assert(a.Metadata.Author, Equals, "somevendor")
		}

	}
}

type aptTranslationArtifactInspectorTest struct {
	filename string // Test artifact filename
	relpath  string // Path in the release file
	sha256   string // File digest
	size     int64  // File size
	result   bool   // expected result
}

var aptTranslationArtifactInspectorTests = []aptTranslationArtifactInspectorTest{{
	filename: "testdata/Translation-en.xz",
	relpath:  "main/i18n/Translation-en.xz",
	sha256:   "a0c8d5a1e5197991564101acca9069e2baa014a9d8ed0ed6143224d752aa1909",
	size:     388,
	result:   true,
}, {
	filename: "testdata/Translation-zh_TW.xz",
	relpath:  "main/i18n/Translation-zh_TW.xz",
	sha256:   "4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed",
	size:     792,
	result:   true,
}, {
	filename: "testdata/Translation-zh_TW.xz",
	sha256:   "4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed",
	size:     600,
	result:   false,
}, {
	filename: "testdata/Translation-zh_TW-bad.xz",
	sha256:   "1b4001d827461c64c63e9b0cba4604e0f494171be2dd310018b456a03f8c6ca5",
	size:     792,
	result:   false,
}, {
	filename: "testdata/Translation-zh_TW-bad.xz",
	sha256:   "1b4001d827461c64c63e9b0cba4604e0f494171be2dd310018b456a03f8c6ca5",
	size:     600,
	result:   false,
}}

func (s *aptSuite) TestAptTranslationArtifactInspector(c *C) {
	for _, tc := range aptTranslationArtifactInspectorTests {
		restorer := apt.MockCheckSignature(func(f io.ReadSeeker, notes Annotation, pubkey string) (io.ReadSeeker, error) {
			return f, nil
		})
		defer restorer()

		var err error

		// Load the release file first
		rel := metadata.NewArtifact()
		rel.RequestInspection = metadata.InspectionMap{
			"apt.release": &Inspection{
				Opinion: opinions.Pending,
				Reason:  "",
				Annotations: Annotation{
					"cfg-name": "default",
				},
			},
		}
		rel.CurrentDownload.URL = "http://archive.ubuntu.com/ubuntu/dists/jammy/InRelease"
		rel.MimeType = mimetype.Lookup("text/plain")
		rel.Metadata.Sha256, err = digests.NewSha256Digest("98e8b22a45d8c663490fcc133384d07534e7c52b49d3f5004a2d87199d4fee5f")
		c.Assert(err, IsNil)

		relFile := strings.NewReader(inReleaseArtifactData)

		// Inspect the InRelease file with the release inspector
		ins := apt.NewAptReleaseInspector(getTestAptConfig())
		err = ins.InspectArtifact(relFile, rel)
		c.Assert(err, IsNil)

		// Now load the translation file
		a := metadata.NewArtifact()
		a.SetRequestPending(ins, "test")
		a.RequestInspection = metadata.InspectionMap{
			"apt.release": &Inspection{
				Opinion: opinions.Pending,
				Reason:  "",
				Annotations: Annotation{
					"cfg-name": "default",
				},
			},
		}
		a.CurrentDownload.URL = "http://archive.ubuntu.com/ubuntu/dists/jammy/main/i18n/by-hash/SHA256/" + tc.sha256
		a.Metadata.Type = "application/x.apt.translation"
		a.Metadata.Size = tc.size
		a.Metadata.Sha256, err = digests.NewSha256Digest(tc.sha256)
		c.Assert(err, IsNil)

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer f.Close()

		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		c.Assert(a.Approved(), Equals, false)

		if tc.result {
			c.Assert(a.Metadata.Type, Equals, "application/x.apt.translation")
			c.Assert(a.ResponseInspection["apt.release"], DeepEquals, &Inspection{
				Opinion: opinions.Unknown,
				Reason:  "Translation file listed in Release",
				Annotations: Annotation{
					"file-path":    tc.relpath,
					"release-file": "98e8b22a45d8c663490fcc133384d07534e7c52b49d3f5004a2d87199d4fee5f",
					"vendor":       "Ubuntu",
				},
			})
		}

	}
}
