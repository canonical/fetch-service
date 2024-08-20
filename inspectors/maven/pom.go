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
	// POM urls have the form /maven2/<org components separated by />/<artefact-id>/<version>/<pom file>
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

func (ins *MavenPomInspector) InspectRequest(a RequestArtefact) error {
	url := a.DownloadURL()
	m := pomRequestOrigin.FindStringSubmatch(url)
	if len(m) == 0 {
		return nil
	}

	if artefactUrl := parseUrl(pomRequestSlug, url); artefactUrl != nil {
		a.SetRequestPending(ins, "request matches valid URL").Annotate(
			Annotation{
				"group-id":    artefactUrl.GroupId,
				"artefact-id": artefactUrl.ArtefactId,
				"version":     artefactUrl.Version,
			},
		)
		return nil
	}
	return nil
}

func (ins *MavenPomInspector) InspectArtefact(f ArtefactReader, a ResponseArtefact) error {
	if !a.MimetypeIs("text/xml") {
		return nil
	}

	artefact_id, ok := a.RequestStringAnnotation(ins.ID(), "artefact-id")
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

	if md.Name == artefact_id && md.Version == version {
		md.Name = fmt.Sprintf(`Maven POM file for '%s'`, artefact_id)
		a.SetArtefactMetadata(*md)
		a.SetResponseApproved(ins, "Maven pom successfully parsed and validated")
	}
	return nil
}
