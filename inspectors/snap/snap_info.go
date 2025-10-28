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

package snap

import (
	"encoding/json"
	"fmt"
	"net/url"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
)

type SnapInfoInspector struct {
}

func NewSnapInfoInspector() *SnapInfoInspector {
	return &SnapInfoInspector{}
}

func (SnapInfoInspector) ID() string {
	return "snap.info"
}

// InspectRequest verifies if the request complies with policy.
func (ins *SnapInfoInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if info, err := newSnapInfoURLInfo(u); err == nil {
		a.SetRequestPending(ins, "valid URL for snap info endpoint").Annotate(
			Annotation{
				"name": info.name,
			},
		)
	}

	return nil // we don't recognize this request
}

type snapInfo struct {
	Name      string            `json:"name"`
	Title     string            `json:"title"`
	Summary   string            `json:"summary"`
	Publisher map[string]string `json:"publisher"`
	SnapID    string            `json:"snap-id"`
}

type snapInfoBody struct {
	ChannelMap   []map[string]any `json:"channel-map"`
	Name         string           `json:"name"`
	DefaultTrack string           `json:"default-track"`
	Snap         snapInfo         `json:"snap"`
	SnapID       string           `json:"snap-id"`
}

// InspectArtifact extracts metadata from a known artifact file format.
func (ins *SnapInfoInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/json") {
		return nil
	}

	decoder := json.NewDecoder(f)
	var b snapInfoBody
	if err := decoder.Decode(&b); err != nil {
		return nil // we don't recognize this artifact
	}

	if len(b.ChannelMap) > 0 && b.ChannelMap[0]["version"] != "" && b.Name != "" && b.SnapID != "" {

		a.SetArtifactMetadata(ArtifactMetadata{
			Type:        mimetypes.SnapInfo,
			Name:        "Store protocol response",
			Description: "Snap store response for info request",
		})
		a.SetResponseApproved(ins, "valid snap API info endpoint response").Annotate(
			Annotation{
				"name":      b.Name,
				"snap-id":   b.SnapID,
				"publisher": b.Snap.Publisher["display-name"],
			})
		return nil
	}

	return nil // we don't recognize this artifact
}
