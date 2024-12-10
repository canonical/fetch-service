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

package maven_test

import (
	"fmt"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/maven"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/gabriel-vasile/mimetype"

	. "github.com/canonical/fetch-service/inspectors/common"
)

func (s *mavenSuite) TestPomInspectorID(c *C) {
	ins := maven.NewPomInspector()
	c.Assert(ins.ID(), Equals, "maven.pom")
}

var pomurltests = []struct {
	// input
	slug string

	// expected output
	group_id    string
	artifact_id string
	version     string
}{
	{"/joda-time/joda-time/2.2/joda-time-2.2.pom", "joda-time", "joda-time", "2.2"},
	{"/apache/maven/maven-artifact/2.0.6/maven-artifact-2.0.6.pom", "apache.maven", "maven-artifact", "2.0.6"},
}

func (s *mavenSuite) TestPomInspectRequest(c *C) {
	for _, pt := range pomurltests {
		ins := maven.NewPomInspector()
		a := metadata.NewArtifact()
		a.CurrentDownload = metadata.Download{URL: "https://repo.maven.apache.org:443/maven2" + pt.slug}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, true)
		c.Assert(insp.Opinion, Equals, opinions.Pending)

		c.Check(a.RequestInspection[ins.ID()].Annotations["group-id"], Equals, pt.group_id)
		c.Check(a.RequestInspection[ins.ID()].Annotations["artifact-id"], Equals, pt.artifact_id)
		c.Check(a.RequestInspection[ins.ID()].Annotations["version"], Equals, pt.version)
	}
}

var pomtests = []struct {
	filename    string
	slug        string
	group_id    string
	artifact_id string
	version     string

	description string
	author      string
	license     string
}{
	{
		"joda-time-pom.xml",
		"/joda-time/joda-time/2.2/joda-time-2.2.pom",
		"joda-time", "joda-time", "2.2",
		"Date and time library to replace JDK date handling",
		"Stephen Colebourne, Brian S O'Neill",
		"Apache 2",
	},
	{
		"plexus-interpolation-pom.xml",
		"/org/codehaus/plexus/plexus-interpolation/1.15/plexus-interpolation-1.15.pom",
		"org.codehaus.plexus", "plexus-interpolation", "1.15",
		"", // no description, author or license in pom
		"",
		"",
	},
}

func (s *mavenSuite) TestPomInspectArtifact(c *C) {
	for _, jt := range pomtests {
		filename := filepath.Join("testdata", jt.filename)

		ins := maven.NewPomInspector()
		h, _ := digests.NewSha1Digest("a5f29a7acaddea3f4af307e8cf2d0cc82645fd7d")

		a := metadata.NewArtifact()
		a.Metadata.Type = "text/xml"
		a.Metadata.Sha1 = h
		a.MimeType = mimetype.Lookup("text/xml")
		a.CurrentDownload.URL = "https://repo.maven.apache.org:443/maven2" + jt.slug
		a.RequestInspection[ins.ID()] = &Inspection{
			Opinion:     opinions.Pending,
			Reason:      "some reason",
			Annotations: Annotation{"group-id": jt.group_id, "artifact-id": jt.artifact_id, "version": jt.version},
		}

		f, err := files.OpenArtifactFile(filename)
		c.Assert(err, IsNil)
		defer f.Close()

		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		c.Assert(a.Approved(), Equals, true)

		c.Check(a.Metadata.Type, Equals, "text/xml")
		c.Check(a.Metadata.Name, Equals, fmt.Sprintf(`Maven POM file for '%s'`, jt.artifact_id))
		c.Check(a.Metadata.Version, Equals, jt.version)
		c.Check(a.Metadata.Description, Equals, jt.description)
		c.Check(a.Metadata.Author, Equals, jt.author)
		c.Check(a.Metadata.License, Equals, jt.license)
		c.Check(a.Metadata.Vendor, Equals, jt.group_id)
	}
}
