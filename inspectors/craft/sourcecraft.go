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

package craft

import (
	"path/filepath"

	"gopkg.in/yaml.v3"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/craft/config"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
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

func (ins *SourcecraftInspector) InspectRequest(a RequestArtifact) error {
	return inspectCraftRequest(ins, a, &ins.config)
}

func (ins *SourcecraftInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if a.ContentType() != "application/x-git-upload-pack-result" {
		return nil
	}

	slog := a.Logger()
	slog.Debugf("Inspecting source artifact")

	checkoutPath, ok := a.ResponseStringAnnotation(GitUploadPackID, "git-checkout-path")
	if !ok {
		// Only the "fetch" command sets git-checkout-path.
		return nil
	}

	slog.Debugf("inspect git upload-pack artifact: checkout at %q", checkoutPath)

	sourcecraftYamlPath := filepath.Join(checkoutPath, "sourcecraft.yaml")
	if _, err := osStat(sourcecraftYamlPath); err != nil {
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
