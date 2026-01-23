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
	"io"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
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

type instanceIndexes map[string]string
type instanceInfos map[string]instanceInfo
type instanceTypes map[string]instanceInfos

func (ins *InstanceTypesInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("text/plain") {
		return nil
	}

	if !a.InspectorRequestOpinionPending(ins) {
		return nil // Not from LXD instance types URL, we don't recognize this artifact
	}

	var data instanceTypes

	recognized := false
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&data); err == nil {
		// Handle consolidated instance types file
		for _, t := range data {
			for _, inst := range t {
				if inst.CPU == "" {
					return nil // Not an instance types file
				}
			}
		}

		recognized = true
	}

	if !recognized {
		// Check if separate instance infos
		_, err := f.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}

		var data instanceInfos
		decoder := yaml.NewDecoder(f)
		if err := decoder.Decode(&data); err == nil {
			for _, inst := range data {
				if inst.CPU == "" {
					return nil // Not an instance types file
				}
			}
			recognized = true
		}
	}

	if !recognized {
		// Handle index file in name: filename.yaml format.
		_, err := f.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}

		var data instanceIndexes
		decoder := yaml.NewDecoder(f)
		if err := decoder.Decode(&data); err == nil {
			for _, v := range data {
				v = strings.TrimSpace(v)
				if !strings.HasSuffix(v, ".yaml") {
					return nil // Not an instance types index file
				}
			}
			recognized = true
		}

	}

	if !recognized {
		return nil
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
