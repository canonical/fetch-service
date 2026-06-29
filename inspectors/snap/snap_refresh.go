// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2026 Canonical Ltd.
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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	return "snap.refresh"
}

// InspectRequest verifies if the request complies with policy.
func (ins *SnapRefreshInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if _, err := newSnapRefreshURLInfo(u); err == nil {
		notes := Annotation{}
		if actions := getSnapRefreshActions(a); len(actions) > 0 {
			notes["refresh-actions"] = actions
		}

		inspection := a.SetRequestPending(ins, "valid URL for snap refresh endpoint")
		if len(notes) > 0 {
			inspection.Annotate(notes)
		}
	}

	return nil // we don't recognize this request
}

type snapData struct {
	Architectures []string `json:"architectures"`
	Version       string   `json:"version"`
	Revision      int      `json:"revision"`
}

type snapRefreshItem struct {
	EffectiveChannel string   `json:"effective-channel"`
	InstanceKey      string   `json:"instance-key"`
	Name             string   `json:"name"`
	ReleasedAt       string   `json:"released-at"`
	Result           string   `json:"result"`
	Snap             snapData `json:"snap"`
	SnapID           string   `json:"snap-id"`
}

type snapRefreshBody struct {
	ErrorList []any             `json:"error-list"`
	Results   []snapRefreshItem `json:"results"`
}

type snapRefreshAction struct {
	Action          string `json:"action,omitempty"`
	Channel         string `json:"channel,omitempty"`
	TrackingChannel string `json:"tracking-channel,omitempty"`
	InstanceKey     string `json:"instance-key,omitempty"`
	Name            string `json:"name,omitempty"`
	SnapID          string `json:"snap-id,omitempty"`
}

func (a snapRefreshAction) requestedTrackingChannel() string {
	if a.TrackingChannel != "" {
		return a.TrackingChannel
	}
	return a.Channel
}

type snapRefreshRequestBody struct {
	Actions []snapRefreshAction `json:"actions"`
}

func getSnapRefreshActions(a RequestArtifact) []snapRefreshAction {
	req := a.HTTPRequest()
	if req == nil || req.Body == nil {
		return nil
	}

	body, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	// Always restore the body so other handlers can still forward/inspect it.
	a.SetRequestBody(io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return nil
	}

	var b snapRefreshRequestBody
	if err := json.Unmarshal(body, &b); err != nil {
		return nil
	}

	return b.Actions
}

func snapRefreshActionsByInstanceKey(actions []snapRefreshAction) map[string]snapRefreshAction {
	if len(actions) == 0 {
		return nil
	}

	byInstanceKey := make(map[string]snapRefreshAction, len(actions))
	for _, action := range actions {
		if action.InstanceKey == "" {
			continue
		}
		byInstanceKey[action.InstanceKey] = action
	}

	return byInstanceKey
}

// InspectArtifact extracts metadata from a known artifact file format.
func (ins *SnapRefreshInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/json") {
		return nil
	}

	decoder := json.NewDecoder(f)
	var b snapRefreshBody
	if err := decoder.Decode(&b); err != nil {
		return nil // we don't recognize this artifact
	}

	if len(b.Results) > 0 && b.Results[0].EffectiveChannel != "" && b.Results[0].Name != "" && b.Results[0].SnapID != "" {
		channel := b.Results[0].EffectiveChannel
		revision := b.Results[0].Snap.Revision
		md := ArtifactMetadata{
			Type:        mimetypes.SnapRefresh,
			Name:        "Store protocol response",
			Description: "Snap store response for refresh request",
		}
		if len(b.Results) == 1 {
			md.ContentID = fmt.Sprintf("%s:%d", channel, revision)
		}

		actions := []snapRefreshAction{}
		if reqActions, ok := a.RequestAnnotation(ins.ID(), "refresh-actions"); ok {
			if val, ok := reqActions.([]snapRefreshAction); ok {
				actions = val
			}
		}

		actionsByInstanceKey := snapRefreshActionsByInstanceKey(actions)

		refreshResults := make([]Annotation, 0, len(b.Results))
		for _, result := range b.Results {
			trackingChannel := ""
			if len(actionsByInstanceKey) > 0 {
				action, ok := actionsByInstanceKey[result.InstanceKey]
				if !ok {
					if result.Result != "download" {
						a.SetResponseRejected(ins, fmt.Sprintf("unexpected instance-key %q for %s result", result.InstanceKey, result.Result), md)
						return nil
					}
				} else {
					if action.Action == "refresh" && result.Result != "refresh" {
						a.SetResponseRejected(ins, fmt.Sprintf("unexpected %s result for refresh action %q", result.Result, result.InstanceKey), md)
						return nil
					}
					if action.Action != "" && action.Action != "refresh" && result.Result == "refresh" {
						a.SetResponseRejected(ins, fmt.Sprintf("unexpected refresh result for %s action %q", action.Action, result.InstanceKey), md)
						return nil
					}
					if action.SnapID != "" && result.SnapID != "" && action.SnapID != result.SnapID {
						a.SetResponseRejected(ins, fmt.Sprintf("refresh result snap-id does not match request action with instance-key %q", result.InstanceKey), md)
						return nil
					}
					trackingChannel = action.requestedTrackingChannel()
				}
			}

			refreshResults = append(refreshResults, Annotation{
				"tracking-channel":  trackingChannel,
				"effective-channel": result.EffectiveChannel,
				"release-timestamp": result.ReleasedAt,
				"result":            result.Result,
				"snap-name":         result.Name,
				"snap-id":           result.SnapID,
				"snap-revision":     result.Snap.Revision,
				"snap-version":      result.Snap.Version,
				"architectures":     result.Snap.Architectures,
			})
		}

		notes := Annotation{
			"refresh-results": refreshResults,
		}

		a.SetResponseApproved(ins, "valid snap API refresh endpoint response", md).Annotate(notes)
	}

	return nil // we don't recognize this artifact
}
