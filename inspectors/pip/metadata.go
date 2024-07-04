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

package pip

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"strings"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type MetadataInspector struct {
}

func NewMetadataInspector() *MetadataInspector {
	return &MetadataInspector{}
}

func (MetadataInspector) ID() string {
	return "pip.metadata"
}

// InspectRequest verifies if the request complies with policy.
func (ins MetadataInspector) InspectRequest(a *metadata.Artefact) error {
	u, err := url.Parse(a.CurrentDownload.URL)
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if checkMetadataUrl(u) == nil {
		a.SetRequestOpinion(ins.ID(), opinions.Pending, "request matches valid URL")
	}

	return nil // we don't recognize this request
}

// InspectArtefact extracts metadata from a known artefact file format.
func (ins *MetadataInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
	if !a.MimeType.Is("text/plain") {
		return nil
	}

	if err := ins.parseMetadataFile(f, a); err != nil {
		return nil // we don't recognize this artefact
	}

	return nil
}

// parseMetadataFile reads metadata entries from the downloaded artefact.
func (ins *MetadataInspector) parseMetadataFile(f io.Reader, a *metadata.Artefact) error {
	sc := bufio.NewScanner(f)
	sc.Split(bufio.ScanLines)

	var mver string
	var name, version, author, email, maintainer string

	md := &a.Metadata

	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			break
		}

		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)

		switch strings.ToLower(k) {
		case "metadata-version":
			mver = v
		case "name":
			name = fmt.Sprintf("metadata file for package '%s'", v)
		case "version":
			version = v
		case "author":
			author = v
		case "author-email":
			email = v
		case "maintainer":
			maintainer = v
		}
	}

	if mver == "" || name == "" || version == "" {
		return nil // not a python metadata file
	}

	md.Type = mimetypes.PythonMetadata
	md.Name = name
	md.Version = version
	md.Description = "Python metadata file"
	md.Author = author
	md.AuthorEmail = email

	if author != "" {
		md.Vendor = author
	} else {
		md.Vendor = maintainer
	}

	a.SetResponseOpinion(ins.ID(), opinions.Approved, "metadata file successfully parsed").Annotate(
		metadata.Annotation{
			"metadata-version": mver,
		},
	)

	return nil
}
