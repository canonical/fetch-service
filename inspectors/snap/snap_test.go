// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2024 Canonical Ltd.
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
	"testing"

	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/snap"
	"github.com/canonical/fetch-service/metadata"
)

type snapSuite struct{}

var _ = Suite(&snapSuite{})

func Test(t *testing.T) { TestingT(t) }

func (s *snapSuite) TestSnapArtefactInspector(c *C) {
	a := metadata.NewArtefact()
	a.Metadata.Type = "application/x.squashfs"
	a.Metadata.Size = 8192

	f, err := files.OpenArtefactFile("tests/UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7.snap")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := snap.NewSnapInspector()
	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, true)
	c.Check(a.Metadata.Type, Equals, "application/x.canonical.snap-package")
	c.Check(a.Metadata.Name, Equals, "word-salad")
	c.Check(a.Metadata.Vendor, Equals, "Alan Pope")
	c.Check(a.Metadata.Size, Equals, int64(8192))
	c.Check(a.Metadata.Version, Equals, "7")
	c.Check(a.Metadata.Architecture, Equals, "amd64")
	c.Check(a.Metadata.Description, Equals, "Word Salad - Password Generator")
	c.Check(a.ResponseInspection["snap"].Annotations, DeepEquals, Annotation{
		"snap-revision-assertion-header": map[string]string{
			"type":              "snap-revision",
			"authority-id":      "canonical",
			"developer-id":      "ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG",
			"sign-key-sha3-384": "BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul",
			"snap-id":           "UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7",
			"snap-revision":     "7",
			"snap-sha3-384":     "v0QSLRBEj2jMuEmtgYJrVjTFArf27nZBIqZrh87mZIF_ph_fmedOwOcZu4wpvLOs",
			"snap-size":         "8192",
			"timestamp":         "2019-02-27T17:30:26.742285Z",
		},
		"snap-declaration-assertion-header": map[string]string{
			"type":              "snap-declaration",
			"authority-id":      "canonical",
			"publisher-id":      "ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG",
			"series":            "16",
			"sign-key-sha3-384": "BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul",
			"snap-id":           "UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7",
			"snap-name":         "word-salad",
			"timestamp":         "2019-02-20T20:17:43.640421Z",
		},
		"account-assertion-header": map[string]string{
			"type":              "account",
			"account-id":        "ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG",
			"authority-id":      "canonical",
			"display-name":      "Alan Pope",
			"revision":          "2118",
			"sign-key-sha3-384": "BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul",
			"timestamp":         "2024-04-11T03:40:37.008746Z",
			"username":          "popey",
			"validation":        "starred",
		},
	})
}
