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

type SnapAuthNonceInspector struct {
}

func NewSnapAuthNonceInspector() *SnapAuthNonceInspector {
	return &SnapAuthNonceInspector{}
}

func (SnapAuthNonceInspector) ID() string {
	return "snap.auth-nonce"
}

// InspectRequest verifies if the request complies with policy.
func (ins *SnapAuthNonceInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if _, err := newSnapAuthNonceURLInfo(u); err != nil {
		return nil // we don't recognize this request
	}

	a.SetRequestPending(ins, "valid URL for snapd authentication nonce")
	return nil
}

type snapAuthNonce struct {
	Nonce string `json:"nonce"`
}

func (ins *SnapAuthNonceInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/json") {
		return nil
	}

	if !a.InspectorRequestOpinionPending(ins) {
		return nil // Not from the snap store, we don't recognize this artifact
	}

	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()

	var data snapAuthNonce
	if err := decoder.Decode(&data); err != nil {
		return nil // we don't recognize this artifact
	}

	if len(data.Nonce) == 0 {
		return nil // we don't recognize this artifact
	}

	a.SetResponseApproved(ins, "valid format for snapd authentication nonce", ArtifactMetadata{
		Type:        mimetypes.SnapAuthNonce,
		Name:        "Authentication nonce",
		Description: "Snapd authentication nonce",
	})

	return nil
}
