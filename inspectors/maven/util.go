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
	"encoding/xml"
	"io"
	"regexp"
	"strings"

	. "github.com/canonical/fetch-service/inspectors/common"
)

type artifactUrl struct {
	ArtifactID string
	GroupID    string
	Version    string
}

func parseUrl(slug *regexp.Regexp, url string) *artifactUrl {
	m := slug.FindStringSubmatch(url)
	if len(m) == 4 {
		// Convert the "/" org separator into "."
		return &artifactUrl{GroupID: strings.ReplaceAll(m[1], "/", "."), ArtifactID: m[2], Version: m[3]}
	}
	return nil
}

// A handy tool in creating these Go structs is https://xml-to-go.github.io/
type project struct {
	ArtifactID  string `xml:"artifactId"`
	GroupID     string `xml:"groupId"`
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
		GroupID string `xml:"groupId"`
		Version string `xml:"version"`
	} `xml:"parent"`
}

func parsePom(tf io.Reader) (*ArtifactMetadata, error) {
	p := project{}
	if err := xml.NewDecoder(tf).Decode(&p); err != nil {
		return nil, err
	}

	md := &ArtifactMetadata{}

	md.Name = p.ArtifactID

	// Maven supports "hierarchical" poms, so if some of the fields are empty
	// try to get them from the parent
	md.Vendor = p.GroupID
	if p.GroupID == "" {
		md.Vendor = p.Parent.GroupID
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
