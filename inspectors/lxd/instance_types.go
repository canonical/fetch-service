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

package lxd

import (
	"fmt"
	"net/url"

	"gopkg.in/yaml.v3"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type InstanceTypesInspector struct {
}

func NewInstanceTypesInspector() *InstanceTypesInspector {
	return &InstanceTypesInspector{}
}

func (InstanceTypesInspector) ID() string {
	return "lxd.instance-types"
}

// InspectRequest verifies if the request complies with policy.
func (ins *InstanceTypesInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if _, err := newInstanceTypesURLInfo(u); err != nil {
		return nil // we don't recognize this request
	}

	a.SetRequestPending(ins, "valid URL for LXD instance types")
	return nil
}

type instanceInfo struct {
	CPU string `yaml:"cpu"`
	Mem string `yaml:"mem"`
}

type instanceTypes map[string]map[string]instanceInfo

func (ins *InstanceTypesInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("text/plain") {
		return nil
	}

	if a.InspectorRequestOpinion(ins) != opinions.Pending {
		return nil // Not from LXD instance types URL, we don't recognize this artifact
	}

	var data instanceTypes

	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&data); err != nil {
		return nil // Not a YAML file
	}

	for _, t := range data {
		for _, inst := range t {
			if inst.CPU == "" {
				return nil // Not an instances types file
			}
		}
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.LXDInstanceTypes,
		Name:        "Instance types",
		Description: "LXD instance types metadata",
	})

	a.SetResponseApproved(ins, "valid LXD instance types metadata").Annotate(
		Annotation{"entries": len(data)},
	)

	return nil
}
