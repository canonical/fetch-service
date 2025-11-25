// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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
	"regexp"

	. "github.com/canonical/fetch-service/inspectors/common"
)

var (
	// Location of the maven repo (will be configurable)
	jarRequestOrigin = regexp.MustCompile(`^https://repo.maven.apache.org:443`)
	// Download urls have the form /maven2/<org components separated by />/<artifact-id>/<version>/<jar file>
	jarRequestSlug = regexp.MustCompile(`/maven2/((?:\w|\-|/)+).*/((?:\w|\-)+)/(.*)/.*.jar$`)
	// Path to pom.xml inside the jar file.
	pomXML = regexp.MustCompile(`^META-INF/maven/([^/]+)/([^/]+)/pom.xml$`)
)

// MavenJarInspector inspects Jar downloaded from a Maven registry.
type MavenJarInspector struct {
}

func NewJarInspector() *MavenJarInspector {
	return &MavenJarInspector{}
}

func (MavenJarInspector) ID() string {
	return "maven.jar"
}

func (ins *MavenJarInspector) InspectRequest(a RequestArtifact) error {
	url := a.DownloadURL()
	m := jarRequestOrigin.FindStringSubmatch(url)
	if len(m) == 0 {
		return nil
	}
	if artifactURL := parseURL(jarRequestSlug, url); artifactURL != nil {
		// Request marked as Unknown because it comes from the default maven.org origin
		a.SetRequestUnknown(ins, "unsupported origin").Annotate(
			Annotation{
				"group-id":    artifactURL.GroupID,
				"artifact-id": artifactURL.ArtifactID,
				"version":     artifactURL.Version,
			},
		)
		return nil
	}
	return nil
}

func (ins *MavenJarInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/jar") {
		return nil
	}

	zf, err := zip.NewReader(f, int64(f.Len()))
	if err != nil {
		return err
	}
	for _, i := range zf.File {
		m := pomXML.FindStringSubmatch(i.Name)
		if m != nil && len(m) == 3 {
			groupID := m[1]
			artifactID := m[2]

			zf, err := i.Open()
			if err != nil {
				return err
			}
			defer zf.Close()

			md, err := parsePom(zf)
			if err != nil {
				return err
			}

			if md.Name == artifactID {
				a.SetArtifactMetadata(*md)
				a.SetResponseApproved(ins, "Maven pom successfully parsed and validated").Annotate(
					Annotation{
						"group-id":    groupID,
						"artifact-id": artifactID,
					},
				)
			}
			break
		}
	}
	return nil
}
