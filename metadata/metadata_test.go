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

package metadata_test

import (
	"encoding/json"
	"testing"

	"github.com/canonical/fetch-service/metadata"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type metadataSuite struct{}

var _ = Suite(&metadataSuite{})

func createMetadata() metadata.Metadata {
	return metadata.Metadata{
		Type:          "my-type",
		Name:          "my-artifact",
		Version:       "1.2.3",
		Vendor:        "Artifact Industries",
		Description:   "A sample artifact",
		Author:        "Artifact A. Uthor",
		AuthorEmail:   "author@example.com",
		Architecture:  "amd64",
		License:       "GPLv3",
		Copyright:     "Copyright Text",
		SourcePackage: "my-source-package",
		AptSuite:      "series-pocket",
		ContentId:     "some-format-specific-identifier",
	}
}

func (s *metadataSuite) TestMetadataJson(c *C) {
	m := createMetadata()

	bytes, err := json.Marshal(m)
	c.Assert(err, IsNil)

	d := make(map[string]any)
	err = json.Unmarshal(bytes, &d)
	c.Assert(err, IsNil)

	c.Check(d["type"], Equals, "my-type")
	c.Check(d["name"], Equals, "my-artifact")
	c.Check(d["version"], Equals, "1.2.3")
	c.Check(d["vendor"], Equals, "Artifact Industries")
	c.Check(d["description"], Equals, "A sample artifact")
	c.Check(d["author"], Equals, "Artifact A. Uthor")
	c.Check(d["author-email"], Equals, "author@example.com")
	c.Check(d["architecture"], Equals, "amd64")
	c.Check(d["license"], Equals, "GPLv3")
	c.Check(d["copyright"], Equals, "Copyright Text")
	c.Check(d["source-package"], Equals, "my-source-package")
	c.Check(d["content-id"], Equals, "some-format-specific-identifier")
	c.Check(d["apt-suite"], Equals, "series-pocket")
}

func (s *metadataSuite) TestMetadataJsonNoSourcePackage(c *C) {
	m := createMetadata()
	m.SourcePackage = ""

	bytes, err := json.Marshal(m)
	c.Assert(err, IsNil)

	d := make(map[string]any)
	err = json.Unmarshal(bytes, &d)
	c.Assert(err, IsNil)

	_, ok := d["source-package"]
	c.Assert(ok, Equals, false)
}
