// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2026 Canonical Ltd.
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

// The BincraftInspector handles upload-pack requests.
// It recognizes "fetch" command from the Git v2 protocol.
type BincraftInspector struct {
	config config.CraftsInspectorConfig
}

func NewBincraftInspector(cfg config.CraftsInspectorConfig) *BincraftInspector {
	return &BincraftInspector{cfg}
}

func (ins *BincraftInspector) ID() string {
	return "craft.bincraft"
}

type bincraftYaml struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Summary string `json:"summary"`
	License string `json:"license,omitempty"`
	Base    string `json:"base"`
}

func (ins *BincraftInspector) InspectRequest(a RequestArtifact) error {
	return inspectCraftRequest(ins, a, &ins.config)
}

func (ins *BincraftInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if a.ContentType() != "application/x-git-upload-pack-result" {
		return nil
	}

	sl := a.Logger()
	sl.Debugf("Inspecting bincraft artifact")

	checkoutPath, ok := a.ResponseStringAnnotation(GitUploadPackID, "git-checkout-path")
	if !ok {
		// this must have been set by the git upload-pack inspector
		a.SetResponseUnknown(ins, "no git checkout found", NoMetadata)
		return nil
	}

	sl.Debugf("inspect git upload-pack artifact: checkout at %q", checkoutPath)

	bincraftYamlPath := filepath.Join(checkoutPath, "bincraft.yaml")
	if _, err := osStat(bincraftYamlPath); err != nil {
		return nil
	}

	md := ArtifactMetadata{
		Type:      mimetypes.Bincraft,
		ContentID: getSingleFetchedRef(a),
	}

	yamlDataFileReader, err := osOpen(bincraftYamlPath)
	if err != nil {
		a.SetResponseRejected(ins, "cannot open bincraft.yaml file", md)
		return nil
	}
	defer yamlDataFileReader.Close()

	var data bincraftYaml
	dec := yaml.NewDecoder(yamlDataFileReader)
	if err := dec.Decode(&data); err != nil {
		a.SetResponseRejected(ins, "cannot decode bincraft.yaml", md)
		return nil
	}

	md.Name = data.Name
	md.Version = data.Version
	md.Description = data.Summary
	md.License = data.License

	a.SetResponseApproved(ins, "bincraft repository found", md)
	return nil
}
