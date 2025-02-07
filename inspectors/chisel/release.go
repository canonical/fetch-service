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
	// Chisel downloads a tarball of a branch of the canonical/chisel-releases
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
	if match == nil || len(match) < 2 {
		// We do not recognize this URL.
		return nil
	}

	branch := match[1]
	a.SetRequestPending(ins, "request matches valid URL").Annotate(
		Annotation{
			"branch-name": branch,
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
//     "chisel-releases-<branch>" directory.
//   - it contains a "slices/" directory inside the "chisel-releases-<branch>"
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

	branch, ok := a.RequestStringAnnotation(ins.ID(), "branch-name")
	if !ok {
		// Following [pip.SimpleIndexInspector].
		return nil
	}

	zf, err := gzip.NewReader(f)
	if err != nil {
		return nil // We do not recognize this artifact.
	}
	defer zf.Close()

	chiselPath := fmt.Sprintf(chiselPathTemplate, branch)
	slicesDir := fmt.Sprintf(slicesDirTemplate, branch)

	// Parse the tarball.
	tr := tar.NewReader(zf)
	md, err := inspectTarball(tr, chiselPath, slicesDir)
	if err != nil {
		if err == errUnrecognized {
			return nil // We do not recognize this artifact.
		}
		return err
	}

	if md.Name == "" {
		// Use the branch name as the metadata name.
		md.Name = branch
	}
	a.SetArtifactMetadata(*md)
	a.SetResponseApproved(ins, "artifact successfully parsed")
	return nil
}

var errUnrecognized = errors.New("unrecognized artifact")

// inspectTarball inspects the artifact tarball and checks that chiselPath and
// slicesDir are present there. It also parses the chisel.yaml file and sets the
// "format" as the metadata version. It returns an error if those files are not
// present or the parsed chisel.yaml is not valid.
func inspectTarball(r *tar.Reader, chiselPath, slicesDir string) (md *ArtifactMetadata, err error) {
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
			return nil, errUnrecognized
		}
		switch h.Name {
		case chiselPath:
			md, err = parseChiselYAML(r)
			if err != nil {
				return nil, fmt.Errorf("cannot parse chisel.yaml: %w", err)
			}
			delete(pending, chiselPath)
		case slicesDir:
			delete(pending, slicesDir)
		}
	}

	if len(pending) > 0 {
		return nil, errUnrecognized
	}

	return md, nil
}

// Only a subset of chisel.yaml fields are parsed here.
type chiselYAML struct {
	Format   string
	Archives map[string]chiselArchive
}

type chiselArchive struct {
	Components []string
	Suites     []string
}

// parseChiselYAML parses the chisel.yaml file and check if it is valid.
// The file is deemed valid if it has a non-empty "format" and at least one
// "archive". Each archive must have at least one "component" and one "suite".
func parseChiselYAML(r io.Reader) (*ArtifactMetadata, error) {
	var data chiselYAML
	dec := yaml.NewDecoder(r)
	if err := dec.Decode(&data); err != nil {
		return nil, err
	}
	if data.Format == "" || len(data.Archives) == 0 {
		return nil, fmt.Errorf("invalid chisel.yaml")
	}
	for _, archive := range data.Archives {
		if len(archive.Components) == 0 || len(archive.Suites) == 0 {
			return nil, fmt.Errorf("invalid chisel.yaml")
		}
	}
	return &ArtifactMetadata{
		Type:    mimetypes.ChiselRelease,
		Version: data.Format,
	}, nil
}
