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

package cargo

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
)

var (
	// Location of the crates repo (will be configurable)
	crateRequestOrigin = regexp.MustCompile(`^https://static.crates.io:443`)
	// Download urls have the form crates/<crate-name>/<crate-version>/download
	crateRequestSlug = regexp.MustCompile(`/crates/([\w-]+)/([\d\.]+)/download$`)
)

// CargoCrateInspector inspects Cargo crates downloaded from a registry.
type CargoCrateInspector struct {
}

func NewCrateInspector() *CargoCrateInspector {
	return &CargoCrateInspector{}
}

func (CargoCrateInspector) ID() string {
	return "cargo.crate"
}

func (ins *CargoCrateInspector) InspectRequest(a RequestArtifact) error {
	url := a.DownloadURL()
	m := crateRequestOrigin.FindStringSubmatch(url)
	if len(m) == 0 {
		return nil
	}
	m = crateRequestSlug.FindStringSubmatch(url)
	if len(m) == 3 {
		packageName := m[1]
		packageVersion := m[2]
		// Request marked as Unknown because it comes from the default crates.io origin
		a.SetRequestUnknown(ins, "unsupported origin").Annotate(
			Annotation{
				"package-name":    packageName,
				"package-version": packageVersion,
			},
		)
		return nil
	}

	return nil
}

func (ins *CargoCrateInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/gzip") {
		return nil
	}

	packageName, ok := a.RequestStringAnnotation(ins.ID(), "package-name")
	if !ok {
		// following SimpleIndexInspector here
		return nil
	}

	packageVersion, ok := a.RequestStringAnnotation(ins.ID(), "package-version")
	if !ok {
		// following SimpleIndexInspector here
		return nil
	}

	sl := a.Logger()

	cargotoml := fmt.Sprintf(`%s-%s/Cargo.toml`, packageName, packageVersion)

	zf, err := gzip.NewReader(f)
	if err != nil {
		return err
	}

	// Parse crate gzip
	tf := tar.NewReader(zf)
	for {
		h, err := tf.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			sl.Debugf("crate tar parsing error: %s", err)
			return nil // we don't recognize this artifact
		}
		if h.Name == cargotoml {
			var md *ArtifactMetadata
			md, err = parseCargoToml(tf)
			if err != nil {
				return err
			}
			// Check that crate metadata matches the request
			if md.Name == packageName && md.Version == packageVersion {
				a.SetResponseApproved(ins, "rust crate successfully parsed", *md)
			}
			break
		}
	}

	return nil
}

type cargoPackage struct {
	Name        string
	Version     string
	Description string
	Authors     []string
	License     string
}

type cargoToml struct {
	Package cargoPackage
}

func parseCargoToml(tf io.Reader) (*ArtifactMetadata, error) {
	crateMD := cargoToml{}

	_, err := toml.NewDecoder(tf).Decode(&crateMD)
	if err != nil {
		return nil, err
	}

	pack := &crateMD.Package

	author := ""
	if len(pack.Authors) > 0 {
		author = strings.Join(pack.Authors, ", ")
	}

	md := ArtifactMetadata{
		Type:        mimetypes.RustCrate,
		Name:        pack.Name,
		Version:     pack.Version,
		Description: strings.TrimSpace(pack.Description),
		Author:      author,
		License:     pack.License,
	}

	return &md, nil
}
