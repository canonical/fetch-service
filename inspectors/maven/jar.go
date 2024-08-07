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

package maven

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"

	. "github.com/canonical/fetch-service/inspectors/common"
)

var (
	// Location of the maven repo (will be configurable)
	jarRequestOrigin = regexp.MustCompile(`^https://repo.maven.apache.org:443`)
	// Download urls have the form /maven2/<org components separated by />/<artifact-id>/<version>/<jar file>
	jarRequestSlug = regexp.MustCompile(`/maven2/((?:\w|\-|/)+).*/((?:\w|\-)+)/(.*)/.*.jar$`)
)

// MavenJarInspector inspects Jar downloaded downloaded from a Maven registry.
type MavenJarInspector struct {
}

func NewJarInspector() *MavenJarInspector {
	return &MavenJarInspector{}
}

func (MavenJarInspector) ID() string {
	return "maven.jar"
}

func (ins *MavenJarInspector) InspectRequest(a RequestArtefact) error {
	url := a.DownloadURL()
	m := jarRequestOrigin.FindStringSubmatch(url)
	if len(m) == 0 {
		return nil
	}
	m = jarRequestSlug.FindStringSubmatch(url)
	if len(m) == 4 {
		// Convert the "/" org separator into "."
		group_id := strings.ReplaceAll(m[1], "/", ".")
		artifact_id := m[2]
		version := m[3]
		a.SetRequestPending(ins, "request matches valid URL").Annotate(
			Annotation{
				"group-id":    group_id,
				"artifact-id": artifact_id,
				"version":     version,
			},
		)
		return nil
	}

	return nil
}

func (ins *MavenJarInspector) InspectArtefact(f ArtefactReader, a ResponseArtefact) error {
	if !a.MimetypeIs("application/jar") {
		return nil
	}

	group_id, ok := a.RequestStringAnnotation(ins.ID(), "group-id")
	if !ok {
		// following SimpleIndexInspector here
		return nil
	}

	artifact_id, ok := a.RequestStringAnnotation(ins.ID(), "artifact-id")
	if !ok {
		return nil
	}

	version, ok := a.RequestStringAnnotation(ins.ID(), "version")
	if !ok {
		return nil
	}

	pom_xml := fmt.Sprintf(`META-INF/maven/%s/%s/pom.xml`, group_id, artifact_id)

	zf, err := zip.NewReader(f, int64(f.Len()))
	if err != nil {
		return err
	}
	for _, i := range zf.File {
		if i.Name == pom_xml {
			zf, err := i.Open()
			if err != nil {
				return err
			}
			defer zf.Close()

			md, err := parsePom(zf)
			if err != nil {
				return err
			}

			if md.Name == artifact_id && md.Version == version {
				a.SetArtefactMetadata(*md)
				a.SetResponseApproved(ins, "Maven pom successfully parsed and validated")
			}
			break
		}
	}
	return nil
}

// A handy tool in creating these Go structs is https://xml-to-go.github.io/
type project struct {
	ArtifactId  string `xml:"artifactId"`
	GroupId     string `xml:"groupId"`
	Version     string `xml:"version"`
	Description string `xml:"description"`
	Developers  struct {
		Developer []struct {
			Name string `xml:"name"`
		} `xml:"developer"`
	} `xml:"developers"`
	Licenses struct {
		License []struct {
			Name string `xml:"name"`
		} `xml:"license"`
	} `xml:"licenses"`
	Parent struct {
		GroupId string `xml:"groupId"`
		Version string `xml:"version"`
	} `xml:"parent"`
}

func parsePom(tf io.Reader) (*ArtefactMetadata, error) {
	p := project{}
	if err := xml.NewDecoder(tf).Decode(&p); err != nil {
		return nil, err
	}

	md := &ArtefactMetadata{}

	md.Name = p.ArtifactId

	// Maven supports "hierarchical" poms, so if some of the fields are empty
	// try to get them from the parent
	md.Vendor = p.GroupId
	if p.GroupId == "" {
		md.Vendor = p.Parent.GroupId
	}

	md.Version = p.Version
	if md.Version == "" {
		md.Version = p.Parent.Version
	}

	md.Description = p.Description

	// Join authors
	authors := make([]string, len(p.Developers.Developer))
	for i, d := range p.Developers.Developer {
		authors[i] = d.Name
	}
	md.Author = strings.Join(authors, ", ")

	// Join licenses
	licenses := make([]string, len(p.Licenses.License))
	for i, l := range p.Licenses.License {
		licenses[i] = l.Name
	}
	// In case of multiple licenses, the maven standard assumes that they are
	// user choice
	// ref: https://maven.apache.org/ref/3-LATEST/maven-model/maven.html
	md.License = strings.Join(licenses, " OR ")

	return md, nil
}
