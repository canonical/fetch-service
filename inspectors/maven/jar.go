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
	"fmt"
	"regexp"

	. "github.com/canonical/fetch-service/inspectors/common"
)

var (
	// Location of the maven repo (will be configurable)
	jarRequestOrigin = regexp.MustCompile(`^https://repo.maven.apache.org:443`)
	// Download urls have the form /maven2/<org components separated by />/<artifact-id>/<version>/<jar file>
	jarRequestSlug = regexp.MustCompile(`/maven2/((?:\w|\-|/)+).*/((?:\w|\-)+)/(.*)/.*.jar$`)
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
	if artifactUrl := parseUrl(jarRequestSlug, url); artifactUrl != nil {
		// Request marked as Unknown because it comes from the default maven.org origin
		a.SetRequestUnknown(ins, "unsupported origin").Annotate(
			Annotation{
				"group-id":    artifactUrl.GroupID,
				"artifact-id": artifactUrl.ArtifactID,
				"version":     artifactUrl.Version,
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

	groupID, ok := a.RequestStringAnnotation(ins.ID(), "group-id")
	if !ok {
		// following SimpleIndexInspector here
		return nil
	}

	artifactID, ok := a.RequestStringAnnotation(ins.ID(), "artifact-id")
	if !ok {
		return nil
	}

	version, ok := a.RequestStringAnnotation(ins.ID(), "version")
	if !ok {
		return nil
	}

	pomXML := fmt.Sprintf(`META-INF/maven/%s/%s/pom.xml`, groupID, artifactID)

	zf, err := zip.NewReader(f, int64(f.Len()))
	if err != nil {
		return err
	}
	for _, i := range zf.File {
		if i.Name == pomXML {
			zf, err := i.Open()
			if err != nil {
				return err
			}
			defer zf.Close()

			md, err := parsePom(zf)
			if err != nil {
				return err
			}

			if md.Name == artifactID && md.Version == version {
				a.SetArtifactMetadata(*md)
				a.SetResponseApproved(ins, "Maven pom successfully parsed and validated")
			}
			break
		}
	}
	return nil
}
