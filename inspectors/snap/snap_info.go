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
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
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
func (ins SnapInfoInspector) InspectRequest(a *metadata.Artefact) error {
	u, err := url.Parse(a.CurrentDownload.URL)
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if info, err := newSnapInfoUrlInfo(u); err == nil {
		a.SetRequestOpinion(ins.ID(), opinions.Pending, "valid URL for snap info endpoint").Annotate(
			metadata.Annotation{
				"name": info.name,
			},
		)
	}

	return nil // we don't recognize this request
}

type snapInfoBody struct {
	ChannelMap []map[string]any `json:"channel-map"`
	Name       string           `json:"name"`
	SnapID     string           `json:"snap-id"`
}

// InspectArtefact extracts metadata from a known artefact file format.
func (ins *SnapInfoInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
	if !a.MimeType.Is("application/json") {
		return nil
	}

	decoder := json.NewDecoder(f)
	var b snapInfoBody
	if err := decoder.Decode(&b); err != nil {
		return nil // we don't recognize this artefact
	}

	if len(b.ChannelMap) > 0 && b.ChannelMap[0]["version"] != "" && b.Name != "" && b.SnapID != "" {
		a.Metadata.Type = mimetypes.SnapInfo
		a.SetResponseOpinion(ins.ID(), opinions.Approved, "valid snap API info endpoint response").Annotate(
			metadata.Annotation{
				"name":    b.Name,
				"snap-id": b.SnapID,
			})
		return nil
	}

	return nil // we don't recognize this artefact
}
