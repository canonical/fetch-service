// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2025 Canonical Ltd.
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

// The CharmcraftInspector handles upload-pack requests.
// It recognizes "fetch" command from the Git v2 protocol.
type CharmcraftInspector struct {
	config config.CraftsInspectorConfig
}

func NewCharmcraftInspector(cfg config.CraftsInspectorConfig) *CharmcraftInspector {
	return &CharmcraftInspector{cfg}
}

func (ins *CharmcraftInspector) ID() string {
	return "craft.charmcraft"
}

type charmcraftYaml struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

func (ins *CharmcraftInspector) InspectRequest(a RequestArtifact) error {
	return inspectCraftRequest(ins, a, &ins.config)
}

func (ins *CharmcraftInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if a.ContentType() != "application/x-git-upload-pack-result" {
		return nil
	}

	slog := a.Logger()
	slog.Debugf("Inspecting artifact")

	checkoutPath, ok := a.ResponseStringAnnotation(GitUploadPackID, "git-checkout-path")
	if !ok {
		// this must have been set by the git upload-pack inspector
		a.SetResponseUnknown(ins, "no git checkout found")
		return nil
	}

	slog.Debugf("inspect git upload-pack artifact: checkout at %q", checkoutPath)

	charmcraftYamlPath := filepath.Join(checkoutPath, "charmcraft.yaml")
	if _, err := osStat(charmcraftYamlPath); err != nil {
		return nil
	}
	yamlFile, err := osOpen(charmcraftYamlPath)
	if err != nil {
		a.SetResponseRejected(ins, "cannot open charmcraft.yaml file")
		return nil
	}
	defer yamlFile.Close()

	var data charmcraftYaml
	dec := yaml.NewDecoder(yamlFile)
	if err := dec.Decode(&data); err != nil {
		a.SetResponseRejected(ins, "cannot decode charmcraft.yaml")
		return nil
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.Charmcraft,
		Name:        data.Name,
		Description: data.Summary,
		ContentId:   getSingleFetchedRef(a),
	})
	a.SetResponseApproved(ins, "charmcraft repository found")
	return nil
}
