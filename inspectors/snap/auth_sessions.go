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

type SnapAuthSessionsInspector struct {
}

func NewSnapAuthSessionsInspector() *SnapAuthSessionsInspector {
	return &SnapAuthSessionsInspector{}
}

func (SnapAuthSessionsInspector) ID() string {
	return "snap.auth-sessions"
}

// InspectRequest verifies if the request complies with policy.
func (ins *SnapAuthSessionsInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if _, err := newSnapAuthSessionsURLInfo(u); err != nil {
		return nil // we don't recognize this request
	}

	a.SetRequestPending(ins, "valid URL for snapd session authentication")
	return nil
}

type snapAuthSessions struct {
	Macaroon string `json:"macaroon"`
}

func (ins *SnapAuthSessionsInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/json") {
		return nil
	}

	if !a.InspectorRequestOpinionPending(ins) {
		return nil // Not from the snap store, we don't recognize this artifact
	}

	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()

	var data snapAuthSessions
	if err := decoder.Decode(&data); err != nil {
		return nil // we don't recognize this artifact
	}

	if len(data.Macaroon) == 0 {
		return nil // we don't recognize this artifact
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.SnapdAuthSessions,
		Name:        "Session authentication",
		Description: "Snapd session authentication",
	})

	a.SetResponseApproved(ins, "valid format for snapd session authentication")

	return nil
}
