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

// The SourcecraftInspector handles upload-pack requests.
// It recognizes "fetch" command from the Git v2 protocol.
type SourcecraftInspector struct {
	config config.CraftsInspectorConfig
}

func NewSourcecraftInspector(cfg config.CraftsInspectorConfig) *SourcecraftInspector {
	return &SourcecraftInspector{cfg}
}

func (ins *SourcecraftInspector) ID() string {
	return "craft.sourcecraft"
}

type sourcecraftYaml struct {
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
func (ins *SourcecraftInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if !checkGitRequestHeaders(a) {
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

	a.SetRequestPending(ins, "valid URL for sourcecraft download")
	return nil
}

func (ins *SourcecraftInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if a.ContentType() != "application/x-git-upload-pack-result" {
		return nil
	}
	logger.Debugf("Inspecting source artifact")

	checkoutPath, ok := a.ResponseStringAnnotation(GitUploadPackID, "git-checkout-path")
	if !ok {
		// this must have been set by the git upload-pack inspector
		a.SetResponseUnknown(ins, "no git checkout found")
		return nil
	}

	logger.Debugf("inspect git upload-pack artifact: checkout at %q", checkoutPath)

	sourcecraftYamlPath := filepath.Join(checkoutPath, "sourcecraft.yaml")
	if _, err := osStat(sourcecraftYamlPath); err != nil {
		a.SetResponseUnknown(ins,
			"git repository does not contain a sourcecraft.yaml file")
		return nil
	}
	yamldata_filereader, err := osOpen(sourcecraftYamlPath)
	if err != nil {
		a.SetResponseRejected(ins, "cannot open sourcecraft.yaml file")
		return nil
	}
	defer yamldata_filereader.Close()

	var data sourcecraftYaml
	dec := yaml.NewDecoder(yamldata_filereader)
	if err := dec.Decode(&data); err != nil {
		a.SetResponseRejected(ins, "cannot decode sourcecraft.yaml")
		return nil
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.Sourcecraft,
		Name:        data.Name,
		Version:     data.Version,
		Description: data.Summary,
		License:     data.License,
	})
	a.SetResponseApproved(ins, "sourcecraft repository found")
	return nil
}
