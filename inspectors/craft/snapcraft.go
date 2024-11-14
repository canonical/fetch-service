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

package craft

import (
	"fmt"
	"net/url"
	"path/filepath"

	"gopkg.in/yaml.v3"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/craft/config"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
)

// The SnapcraftInspector handles upload-pack requests.
// It recognizes "fetch" command from the Git v2 protocol.
type SnapcraftInspector struct {
	config config.CraftsInspectorConfig
}

func NewSnapcraftInspector(cfg config.CraftsInspectorConfig) *SnapcraftInspector {
	return &SnapcraftInspector{cfg}
}

func (ins *SnapcraftInspector) ID() string {
	return "craft.snapcraft"
}

type snapcraftYaml struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Summary string `json:"summary"`
	License string `json:"license,omitempty"`
	Base    string `json:"base"`
}

// InspectRequest verifies whether this is a valid upload-pack request. For
// it to succeed the following conditions must be satisfied:
//
//   - The "Git-Protocol" request header must be set to "version=2".
//   - The Content-Type header must be set to "application/x-git-upload-pack-request".
//   - The Accept header must be set to "application/x-git-upload-pack-result"
//   - The request URL must match a valid upload-pack pattern.
//   - The upload-pack command must be "fetch".
//   - It must be a shallow fetch.
func (ins *SnapcraftInspector) InspectRequest(a RequestArtefact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	content_type, ok := a.RequestHeader("Content-Type")
	if !ok || len(content_type) < 1 || content_type[0] != "application/x-git-upload-pack-request" {
		return nil // we don't recognize this request
	}

	accept, ok := a.RequestHeader("Accept")
	if !ok || len(accept) < 1 || accept[0] != "application/x-git-upload-pack-result" {
		return nil // we don't recognize this request
	}

	_, err = config.NewCraftUrlInfo(u, &ins.config)
	if err != nil {
		return nil // we don't recognize this request
	}

	command, ok := a.RequestStringAnnotation(GitUploadPackID, "command")
	if !ok || command != "fetch" {
		return nil // we don't recognize this request
	}

	a.SetRequestPending(ins, "valid URL for snapcraft download")
	return nil
}

func (ins *SnapcraftInspector) InspectArtefact(f ArtefactReader, a ResponseArtefact) error {
	if a.ContentType() != "application/x-git-upload-pack-result" {
		return nil
	}
	logger.Debugf("Inspecting snapcraft artefact")

	checkoutPath, ok := a.ResponseStringAnnotation(GitUploadPackID, "git-checkout-path")
	if !ok {
		// this must have been set by the git upload-pack inspector
		a.SetResponseUnknown(ins, "no git checkout found")
		return nil
	}

	logger.Debugf("inspect git upload-pack artefact: checkout at %q", checkoutPath)

	snapcraftYamlPath, found := getSnapcraftYamlPath(checkoutPath)
	if !found {
		a.SetResponseUnknown(ins,
			"git repository does not contain a snapcraft.yaml file")
		return nil
	}
	yamldata_filereader, err := osOpen(snapcraftYamlPath)
	if err != nil {
		a.SetResponseRejected(ins, "cannot open snapcraft.yaml file")
		return nil
	}
	defer yamldata_filereader.Close()

	var data snapcraftYaml
	dec := yaml.NewDecoder(yamldata_filereader)
	if err := dec.Decode(&data); err != nil {
		a.SetResponseRejected(ins, "cannot decode snapcraft.yaml")
		return nil
	}

	a.SetArtefactMetadata(ArtefactMetadata{
		Type:        mimetypes.Snapcraft,
		Name:        data.Name,
		Version:     data.Version,
		Description: data.Summary,
		License:     data.License,
	})
	a.SetResponseApproved(ins, "snapcraft repository found")

	return nil
}

func getSnapcraftYamlPath(dir string) (path string, found bool) {
	candidates := []string{
		"snap/snapcraft.yaml",
		"snapcraft.yaml",
		"build-aux/snap/snapcraft.yaml",
	}

	for _, c := range candidates {
		p := filepath.Join(dir, c)
		if _, err := osStat(p); err == nil {
			return p, true
		}
	}

	return "", false
}
