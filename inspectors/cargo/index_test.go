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

package cargo_test

import (
	"os"
	"path/filepath"

	"github.com/canonical/fetch-service/inspectors/cargo"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/gabriel-vasile/mimetype"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
	. "gopkg.in/check.v1"
)

var config_url = "https://index.crates.io:443/config.json"
var crate_url = "https://index.crates.io:443/ti/me/time"

func (s *cargoSuite) TestIndexInspectorID(c *C) {
	ins := cargo.NewIndexInspector()
	c.Assert(ins.ID(), Equals, "cargo.index")
}

func (s *cargoSuite) TestIndexInspectRequestBadOrigin(c *C) {
	ins := cargo.NewIndexInspector()
	a := metadata.NewArtifact()
	a.CurrentDownload = metadata.Download{URL: "https://bad.index.io:443/config.json"}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)

	_, ok := a.RequestInspection[ins.ID()]
	c.Assert(ok, Equals, false)
}

func (s *cargoSuite) TestIndexInspectRequestBadSlug(c *C) {
	ins := cargo.NewIndexInspector()
	a := metadata.NewArtifact()
	a.CurrentDownload = metadata.Download{URL: "https://index.crates.io:443/bad/package/name"}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)

	_, ok := a.RequestInspection[ins.ID()]
	c.Assert(ok, Equals, false)
}

func (s *cargoSuite) TestIndexInspectRequestConfig(c *C) {
	ins := cargo.NewIndexInspector()
	a := metadata.NewArtifact()
	a.CurrentDownload = metadata.Download{URL: config_url}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)

	insp, ok := a.RequestInspection[ins.ID()]
	c.Assert(ok, Equals, true)
	c.Assert(insp.Opinion, Equals, opinions.Pending)

	c.Assert(a.RequestInspection[ins.ID()].Annotations["is-config"], Equals, true)
	c.Assert(a.RequestInspection[ins.ID()].Annotations["crate-name"], Equals, "")
}

func (s *cargoSuite) TestIndexInspectRequestCrate(c *C) {
	ins := cargo.NewIndexInspector()
	a := metadata.NewArtifact()
	a.CurrentDownload = metadata.Download{URL: crate_url}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)

	insp, ok := a.RequestInspection[ins.ID()]
	c.Assert(ok, Equals, true)
	c.Assert(insp.Opinion, Equals, opinions.Unknown)
	c.Assert(insp.Reason, Equals, "unsupported origin")

	c.Assert(a.RequestInspection[ins.ID()].Annotations["is-config"], Equals, false)
	c.Assert(a.RequestInspection[ins.ID()].Annotations["crate-name"], Equals, "time")
}

func (s *cargoSuite) TestIndexInspectArtifactConfig(c *C) {
	tmp := c.MkDir()
	filename := filepath.Join(tmp, "config.json")
	contents := "{\n" +
		`    "dl": "https://static.crates.io/crates",` + "\n" +
		`    "api": "https://crates.io"` + "\n" +
		"}\n"
	err := os.WriteFile(filename, []byte(contents),
		0755,
	)
	c.Assert(err, IsNil)

	ins := cargo.NewIndexInspector()
	h, _ := digests.NewSha1Digest("85fc2d2a3764089191e57cd552601278a5985c46")

	a := metadata.NewArtifact()
	a.Metadata.Type = "application/json"
	a.Metadata.Sha1 = h
	a.MimeType = mimetype.Lookup("application/json")
	a.CurrentDownload.URL = config_url
	a.RequestInspection[ins.ID()] = &Inspection{
		Opinion:     opinions.Pending,
		Reason:      "some reason",
		Annotations: Annotation{"is-config": true, "crate-name": ""},
	}

	f, err := files.OpenArtifactFile(filename)
	c.Assert(err, IsNil)
	defer f.Close()

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, true)

	c.Check(a.Metadata.Type, Equals, "application/json")
	c.Check(a.Metadata.Name, Equals, "config.json for Cargo package index")
	c.Check(a.Metadata.Vendor, Equals, "https://crates.io")
	c.Check(a.Metadata.Description, Equals, "config.json for Cargo package index")
	c.Check(a.Metadata.Author, Equals, "https://crates.io")
	c.Check(a.Metadata.AuthorEmail, Equals, "")
	c.Check(a.Metadata.License, Equals, "")
	c.Assert(a.Approved(), Equals, true)
}

func (s *cargoSuite) TestIndexInspectArtifactCrate(c *C) {
	filename := filepath.Join("testdata", "time.ndjson")

	ins := cargo.NewIndexInspector()
	h, _ := digests.NewSha1Digest("85fc2d2a3764089191e57cd552601278a5985c46")

	a := metadata.NewArtifact()
	a.Metadata.Type = "application/x-ndjson"
	a.Metadata.Sha1 = h
	a.MimeType = mimetype.Lookup("application/x-ndjson")
	a.CurrentDownload.URL = config_url
	a.RequestInspection[ins.ID()] = &Inspection{
		Opinion:     opinions.Pending,
		Reason:      "some reason",
		Annotations: Annotation{"is-config": false, "crate-name": "time"},
	}

	f, err := files.OpenArtifactFile(filename)
	c.Assert(err, IsNil)
	defer f.Close()

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, true)

	c.Check(a.Metadata.Type, Equals, "application/x-ndjson")
	c.Check(a.Metadata.Name, Equals, "time")
	c.Check(a.Metadata.Description, Equals, `Cargo package index for crate "time"`)
	c.Assert(a.Approved(), Equals, true)
}
