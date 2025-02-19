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

package chisel

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"regexp"

	"gopkg.in/yaml.v3"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
)

// The Chisel release inspector verifies the chisel-releases repository's [1]
// tarball download request and response artifact.
//
// Chisel [2] downloads the tarball via a GET request to
// https://codeload.github.com. The inspector monitors this request and
// currently only examines the gzip compressed tarball and checks if it contains
// appropriate files.
//
// [1] https://github.com/canonical/chisel-releases
// [2] https://github.com/canonical/chisel
type ChiselReleaseInspector struct{}

func NewChiselReleaseInspector() *ChiselReleaseInspector {
	return &ChiselReleaseInspector{}
}

func (ins *ChiselReleaseInspector) ID() string {
	return "chisel.release"
}

const (
	baseURL = "https://codeload.github.com:443/canonical/chisel-releases/tar.gz/refs/heads"
	slugExp = `([a-z](?:-?[a-z0-9]){2,}-[0-9]+(?:\.?[0-9])+)`
)

var (
	// Chisel downloads a tarball of a branch (release) of the chisel-releases
	// repository, using the codeload.github.com service.
	// See https://github.com/canonical/chisel/blob/cdd1a3c22ac99a948d300d5b36ae6218f964af85/internal/setup/fetch.go#L30
	requestOrigin = regexp.MustCompile(
		fmt.Sprintf("^%s/%s$", baseURL, slugExp),
	)
)

// InspectRequest verifies whether this is a valid chisel-release fetch request.
// For it to succeed the following conditions must be satisfied:
//
//   - The URL must match with [requestOrigin].
func (ins *ChiselReleaseInspector) InspectRequest(a RequestArtifact) error {
	url := a.DownloadURL()
	match := requestOrigin.FindStringSubmatch(url)
	if len(match) < 2 {
		// We do not recognize this URL.
		return nil
	}

	release := match[1]
	a.SetRequestPending(ins, "request matches valid URL").Annotate(
		Annotation{
			"release": release,
		},
	)
	return nil
}

// These paths must be present in the artifact.
var (
	chiselPathTemplate = "chisel-releases-%s/chisel.yaml"
	slicesDirTemplate  = "chisel-releases-%s/slices/"
)

// InspectArtifact sets an artifact to approved if:
//   - it is a gzip compressed tarball.
//   - it contains a valid "chisel.yaml" file in the top-level
//     "chisel-releases-<release>" directory.
//   - it contains a "slices/" directory inside the "chisel-releases-<release>"
//     directory.
//
// It only rejects an artifact if there is chisel.yaml file inside, but the file
// contents are not valid.
//
// In all other cases, it returns nil, which indicates that it does not
// recognize the artifact.
func (ins *ChiselReleaseInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/gzip") {
		return nil // We do not recognize this artifact.
	}

	release, ok := a.RequestStringAnnotation(ins.ID(), "release")
	if !ok {
		// Following [pip.SimpleIndexInspector].
		return nil
	}

	zf, err := gzip.NewReader(f)
	if err != nil {
		return nil // We do not recognize this artifact.
	}
	defer zf.Close()

	chiselPath := fmt.Sprintf(chiselPathTemplate, release)
	slicesDir := fmt.Sprintf(slicesDirTemplate, release)

	// Parse the tarball.
	tr := tar.NewReader(zf)
	format, err := inspectTarball(tr, chiselPath, slicesDir)
	if err != nil {
		if err == errUnrecognized {
			return nil // We do not recognize this artifact.
		}
		return err
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.ChiselRelease,
		Name:        release,
		Version:     format,
		Description: fmt.Sprintf("Chisel release file for %s", release),
		Vendor:      "Canonical",
	})
	a.SetResponseApproved(ins, "artifact successfully parsed")
	return nil
}

var errUnrecognized = errors.New("unrecognized artifact")

// inspectTarball inspects the artifact tarball and checks that chiselPath and
// slicesDir are present there.
// It parses the chisel.yaml file and returns the "format". It returns an error
// if those files are not present or the parsed chisel.yaml is not valid.
func inspectTarball(r *tar.Reader, chiselPath, slicesDir string) (format string, err error) {
	pending := map[string]bool{
		chiselPath: true,
		slicesDir:  true,
	}

	for {
		h, err := r.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", errUnrecognized
		}
		switch h.Name {
		case chiselPath:
			format, err = parseChiselYAML(r)
			if err != nil {
				return "", fmt.Errorf("cannot parse chisel.yaml: %w", err)
			}
			delete(pending, chiselPath)
		case slicesDir:
			delete(pending, slicesDir)
		}
	}

	if len(pending) > 0 {
		return "", errUnrecognized
	}

	return format, nil
}

// Only a subset of chisel.yaml fields are parsed here.
type chiselYAML struct {
	Format     string
	Archives   map[string]chiselArchive
	PublicKeys map[string]chiselPublicKey
}

type chiselArchive struct {
	Components []string
	Suites     []string
	PublicKeys []string
}

type chiselPublicKey struct {
	ID    string
	Armor string
}

// parseChiselYAML parses the chisel.yaml file and check if it is valid.
// The file is deemed valid if it has a non-empty "format" and at least one
// "archive". Each archive must have at least one "component" and one "suite".
func parseChiselYAML(r io.Reader) (format string, err error) {
	var data chiselYAML
	dec := yaml.NewDecoder(r)
	if err = dec.Decode(&data); err != nil {
		return "", err
	}
	if data.Format == "" || len(data.Archives) == 0 || len(data.PublicKeys) == 0 {
		return "", fmt.Errorf("invalid chisel.yaml")
	}
	for _, archive := range data.Archives {
		if len(archive.Components) == 0 || len(archive.Suites) == 0 ||
			len(archive.PublicKeys) == 0 {
			return "", fmt.Errorf("invalid chisel.yaml")
		}
		for _, key := range archive.PublicKeys {
			if _, ok := data.PublicKeys[key]; !ok {
				return "", fmt.Errorf("invalid chisel.yaml")
			}
		}
	}
	return data.Format, nil
}
