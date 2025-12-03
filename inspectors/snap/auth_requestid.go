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

type SnapAuthRequestIDInspector struct {
}

func NewSnapAuthRequestIDInspector() *SnapAuthRequestIDInspector {
	return &SnapAuthRequestIDInspector{}
}

func (SnapAuthRequestIDInspector) ID() string {
	return "snap.auth-request-id"
}

// InspectRequest verifies if the request complies with policy.
func (ins *SnapAuthRequestIDInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if _, err := newSnapAuthRequestIDURLInfo(u); err != nil {
		return nil // we don't recognize this request
	}

	a.SetRequestPending(ins, "valid URL for snapd device authentication request ID")
	return nil
}

type snapAuthRequestID struct {
	RequestID string `json:"request-id"`
}

func (ins *SnapAuthRequestIDInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/json") {
		return nil
	}

	if !a.InspectorRequestOpinionPending(ins) {
		return nil // Not from the snap store, we don't recognize this artifact
	}

	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()

	var data snapAuthRequestID
	if err := decoder.Decode(&data); err != nil {
		return nil // we don't recognize this artifact
	}

	if len(data.RequestID) == 0 {
		return nil // we don't recognize this artifact
	}

	a.SetResponseApproved(ins, "valid format for snapd device authentication request ID", ArtifactMetadata{
		Type:        mimetypes.SnapAuthRequestID,
		Name:        "Device authentication request ID",
		Description: "Snapd device authentication request ID",
	})

	return nil
}
