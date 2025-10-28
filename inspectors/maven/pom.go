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
	"fmt"
	"regexp"

	. "github.com/canonical/fetch-service/inspectors/common"
)

var (
	// Location of the maven repo (will be configurable)
	pomRequestOrigin = regexp.MustCompile(`^https://repo.maven.apache.org:443`)
	// POM urls have the form /maven2/<org components separated by />/<artifact-id>/<version>/<pom file>
	pomRequestSlug = regexp.MustCompile(`/maven2/((?:\w|\-|/)+).*/((?:\w|\-)+)/(.*)/.*.pom$`)
)

// MavenPomInspector inspects Maven POM files downloaded from a Maven registry.
type MavenPomInspector struct {
}

func NewPomInspector() *MavenPomInspector {
	return &MavenPomInspector{}
}

func (MavenPomInspector) ID() string {
	return "maven.pom"
}

func (ins *MavenPomInspector) InspectRequest(a RequestArtifact) error {
	url := a.DownloadURL()
	m := pomRequestOrigin.FindStringSubmatch(url)
	if len(m) == 0 {
		return nil
	}

	if artifactUrl := parseUrl(pomRequestSlug, url); artifactUrl != nil {
		// Request marked as Unknown because it comes from the default maven.org origin
		a.SetRequestUnknown(ins, "unsupported origin").Annotate(
			Annotation{
				"group-id":    artifactUrl.GroupId,
				"artifact-id": artifactUrl.ArtifactId,
				"version":     artifactUrl.Version,
			},
		)
		return nil
	}
	return nil
}

func (ins *MavenPomInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("text/xml") {
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

	md, err := parsePom(f)
	if err != nil {
		return err
	}

	if md.Name == artifactID && md.Version == version {
		md.Name = fmt.Sprintf(`Maven POM file for '%s'`, artifactID)
		a.SetArtifactMetadata(*md)
		a.SetResponseApproved(ins, "Maven pom successfully parsed and validated")
	}
	return nil
}
