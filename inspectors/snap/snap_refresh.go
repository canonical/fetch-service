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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"sync"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
)

type SnapRefreshInspector struct {
	mu      sync.Mutex
	results map[string]snapRefreshResolution
}

func NewSnapRefreshInspector() *SnapRefreshInspector {
	return &SnapRefreshInspector{results: make(map[string]snapRefreshResolution)}
}

func (*SnapRefreshInspector) ID() string {
	return "snap.refresh"
}

// InspectRequest verifies if the request complies with policy.
func (ins *SnapRefreshInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if _, err := newSnapRefreshURLInfo(u); err == nil {
		annotations, err := inspectSnapRefreshRequestBody(a)
		if err != nil {
			return err
		}
		inspection := a.SetRequestPending(ins, "valid URL for snap refresh endpoint")
		if len(annotations) > 0 {
			inspection.Annotate(annotations)
		}
		return nil
	}

	if info, err := newSnapPackageURLInfo(u); err == nil {
		if res, ok := ins.lookupResolution(info.snapID, info.release); ok {
			a.SetRequestPending(ins, "snap download matches prior refresh result").Annotate(
				Annotation{
					"name":              res.Name,
					"version":           res.Version,
					"snap-id":           res.SnapID,
					"revision":          res.Revision,
					"requested-channel": res.ReqChannel,
					"effective-channel": res.Channel,
				},
			)
		}
	}

	return nil // we don't recognize this request
}

type snapRefreshResolution struct {
	ReqChannel string
	Channel    string
	SnapID     string
	Revision   int
	Name       string
	Version    string
	Result     string
}

func snapRefreshResolutionKey(snapID string, revision int) string {
	return fmt.Sprintf("%s:%d", snapID, revision)
}

func (ins *SnapRefreshInspector) lookupResolution(snapID, revision string) (snapRefreshResolution, bool) {
	rev, err := strconv.Atoi(revision)
	if err != nil {
		return snapRefreshResolution{}, false
	}
	ins.mu.Lock()
	defer ins.mu.Unlock()
	resolution, ok := ins.results[snapRefreshResolutionKey(snapID, rev)]
	return resolution, ok
}

func (ins *SnapRefreshInspector) storeResolution(result snapRefreshResolution) {
	ins.mu.Lock()
	defer ins.mu.Unlock()
	ins.results[snapRefreshResolutionKey(result.SnapID, result.Revision)] = result
}

type snapRefreshRequest struct {
	Actions []snapRefreshRequestItem `json:"actions"`
}

type snapRefreshRequestItem struct {
	Channel string `json:"channel"`
}

func inspectSnapRefreshRequestBody(a RequestArtifact) (Annotation, error) {
	req := a.HTTPRequest()
	if req == nil || req.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("cannot read snap refresh request body: %w", err)
	}
	if cerr := req.Body.Close(); cerr != nil {
		a.Logger().Debugf("snap refresh: cannot close request body: %s", cerr)
	}
	a.SetRequestBody(io.NopCloser(bytes.NewReader(body)))

	if len(body) == 0 {
		return nil, nil
	}

	var payload snapRefreshRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, nil
	}

	for _, item := range payload.Actions {
		if item.Channel != "" {
			return Annotation{"requested-channel": item.Channel}, nil
		}
	}

	return nil, nil
}

type snapData struct {
	Version  string `json:"version"`
	Revision int    `json:"revision"`
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

// InspectArtifact extracts metadata from a known artifact file format.
func (ins *SnapRefreshInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if a.MimetypeIs(mimetypes.SquashFs) {
		if res, ok := ins.getSnapDownloadResolution(a); ok {
			a.SetResponseUnknown(ins, "snap download matches prior refresh result", NoMetadata).Annotate(
				Annotation{
					"name":              res.Name,
					"snap-id":           res.SnapID,
					"version":           res.Version,
					"revision":          res.Revision,
					"requested-channel": res.ReqChannel,
					"effective-channel": res.Channel,
				},
			)
		}
		return nil
	}

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
		requestedChannel, _ := a.RequestStringAnnotation(ins.ID(), "requested-channel")
		ins.storeResolution(snapRefreshResolution{
			SnapID:     b.Results[0].SnapID,
			Revision:   revision,
			Name:       b.Results[0].Name,
			Version:    b.Results[0].Snap.Version,
			Result:     b.Results[0].Result,
			ReqChannel: requestedChannel,
			Channel:    channel,
		})
		notes := Annotation{
			"name":     b.Results[0].Name,
			"version":  b.Results[0].Snap.Version,
			"revision": revision,
			"channel":  channel,
			"result":   b.Results[0].Result,
			"snap-id":  b.Results[0].SnapID,
		}

		a.SetResponseApproved(ins, "valid snap API refresh endpoint response", ArtifactMetadata{
			Type:        mimetypes.SnapRefresh,
			Name:        "Store protocol response",
			Description: "Snap store response for refresh request",
			ContentID:   fmt.Sprintf("%s:%d", channel, revision),
		}).Annotate(notes)
	}

	return nil // we don't recognize this artifact
}

func (ins *SnapRefreshInspector) getSnapDownloadResolution(a ResponseArtifact) (snapRefreshResolution, bool) {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return snapRefreshResolution{}, false
	}
	info, err := newSnapPackageURLInfo(u)
	if err != nil {
		return snapRefreshResolution{}, false
	}
	return ins.lookupResolution(info.snapID, info.release)
}
