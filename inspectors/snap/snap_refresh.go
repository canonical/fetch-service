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

type SnapRefreshInspector struct {
}

func NewSnapRefreshInspector() *SnapRefreshInspector {
	return &SnapRefreshInspector{}
}

func (SnapRefreshInspector) ID() string {
	return "snap.info"
}

// InspectRequest verifies if the request complies with policy.
func (ins *SnapRefreshInspector) InspectRequest(a RequestArtefact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if _, err := newSnapRefreshUrlInfo(u); err == nil {
		a.SetRequestPending(ins, "valid URL for snap refresh endpoint")
	}

	return nil // we don't recognize this request
}

type snapRefreshBody struct {
	Results []map[string]any `json:"results"`
}

// InspectArtefact extracts metadata from a known artefact file format.
func (ins *SnapRefreshInspector) InspectArtefact(f ArtefactReader, a ResponseArtefact) error {
	if !a.MimetypeIs("application/json") {
		return nil
	}

	decoder := json.NewDecoder(f)
	var b snapRefreshBody
	if err := decoder.Decode(&b); err != nil {
		return nil // we don't recognize this artefact
	}

	if len(b.Results) > 0 && b.Results[0]["effective-channel"] != "" && b.Results[0]["name"] != "" && b.Results[0]["snap-id"] != "" {
		a.SetArtefactMetadata(ArtefactMetadata{
			Type:        mimetypes.SnapInfo,
			Name:        "Store protocol response",
			Description: "Snap store response for refresh request",
		})
		a.SetResponseApproved(ins, "valid snap API refresh endpoint response").Annotate(
			Annotation{
				"name":    b.Results[0]["name"],
				"snap-id": b.Results[0]["snap-id"],
			})
		return nil
	}

	return nil // we don't recognize this artefact
}
