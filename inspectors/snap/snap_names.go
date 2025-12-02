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

package snap

import (
	"encoding/json"
	"fmt"
	"net/url"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
)

type SnapNamesInspector struct {
}

func NewSnapNamesInspector() *SnapNamesInspector {
	return &SnapNamesInspector{}
}

func (SnapNamesInspector) ID() string {
	return "snap.names"
}

// InspectRequest verifies if the request complies with policy.
func (ins *SnapNamesInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if _, err := newSnapNamesURLInfo(u); err != nil {
		return nil // we don't recognize this request
	}

	a.SetRequestPending(ins, "valid URL for Snap package names list")
	return nil
}

type aliasInfo struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

type packageInfo struct {
	Aliases     []aliasInfo `json:"aliases"`
	Apps        []string    `json:"apps"`
	PackageName string      `json:"package-name"`
	Summary     string      `json:"summary"`
	Title       string      `json:"title"`
	Version     string      `json:"version"`
}

type embeddedInfo struct {
	PackageList []packageInfo `json:"clickindex:package"`
}

type namesResult struct {
	Embedded embeddedInfo `json:"_embedded"`
}

func (ins *SnapNamesInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/json") {
		return nil
	}

	if !a.InspectorRequestOpinionPending(ins) {
		return nil // Not from the snap store, we don't recognize this artifact
	}

	decoder := json.NewDecoder(f)

	var data namesResult
	if err := decoder.Decode(&data); err != nil {
		return nil // we don't recognize this artifact
	}

	if data.Embedded.PackageList == nil {
		return nil // we don't recognize this artifact
	}

	num := len(data.Embedded.PackageList)
	if num == 0 {
		return nil // we don't recognize this artifact
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.SnapNames,
		Name:        "Snap names list",
		Description: "List of Snap package names",
	})

	a.SetResponseApproved(ins, "valid Snap package names list").Annotate(
		Annotation{"entries": num},
	)

	return nil
}
