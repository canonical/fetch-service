// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023 Canonical Ltd.
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

	"github.com/canonical/fetch-service/inspectors/apt"
	"github.com/canonical/fetch-service/metadata"
	. "gopkg.in/check.v1"
)

// XXX: This file contains minimal testing for apt file formats. Tests
//
//	will be extended after the metadata format is approved.

func (s *aptSuite) TestAptTranslationsInspector(c *C) {

	for _, tc := range []struct {
		dataFile          string
		result            bool
		lang              string
		translataionCount int
	}{
		{"tests/Translation-zh_TW.xz", true, "zh_TW", 3},
		{"tests/Translation-zh_TW-bad.xz", false, "", 0},
	} {
		translationArtefactFile, _ := os.Open(tc.dataFile)
		translationArtefactData := make([]byte, 1024*128)
		_, err := translationArtefactFile.Read(translationArtefactData)

		c.Assert(err, IsNil)

		ins := apt.NewAptTranslationsInspector()
		t := metadata.NewArtefact()
		t.CurrentDownload.URL = "http://archive.ubuntu.com/ubuntu/dists/devel/main/i18n/by-hash/SHA256/4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed"
		t.Metadata.Type = "application/x.apt.translation"
		t.Metadata.Sha256, _ = metadata.NewSha256Digest("4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed")

		f := bytes.NewReader(translationArtefactData)

		err = ins.InspectArtefact(f, t)

		c.Assert(err, IsNil)
		c.Assert(t.Approved(), Equals, tc.result)
		if tc.result {
			c.Assert(t.ResponseInspection[ins.ID()].Annotations["translation-language"], Equals, tc.lang)
			c.Assert(t.ResponseInspection[ins.ID()].Annotations["translation-count"], Equals, tc.translataionCount)
		}

	}
}
