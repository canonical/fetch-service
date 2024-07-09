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
func (ins *MetadataInspector) InspectRequest(a RequestArtefact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if checkMetadataUrl(u) == nil {
		a.SetRequestPending(ins, "request matches valid URL")
	}

	return nil // we don't recognize this request
}

// InspectArtefact extracts metadata from a known artefact file format.
func (ins *MetadataInspector) InspectArtefact(f ArtefactReader, a ResponseArtefact) error {
	if !a.MimetypeIs("text/plain") {
		return nil
	}

	if err := ins.parseMetadataFile(f, a); err != nil {
		return nil // we don't recognize this artefact
	}

	return nil
}

// parseMetadataFile reads metadata entries from the downloaded artefact.
func (ins *MetadataInspector) parseMetadataFile(f io.Reader, a ResponseArtefact) error {
	sc := bufio.NewScanner(f)
	sc.Split(bufio.ScanLines)

	var mver string
	var name, version, author, email, maintainer, vendor string

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

	if author != "" {
		vendor = author
	} else {
		vendor = maintainer
	}

	a.SetArtefactMetadata(ArtefactMetadata{
		Type:        mimetypes.PythonMetadata,
		Name:        name,
		Version:     version,
		Description: "Python metadata file",
		Author:      author,
		AuthorEmail: email,
		Vendor:      vendor,
	})

	a.SetResponseApproved(ins, "metadata file successfully parsed").Annotate(
		Annotation{
			"metadata-version": mver,
		},
	)

	return nil
}
