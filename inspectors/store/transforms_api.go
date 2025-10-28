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

package store

import (
	"encoding/json"
	"fmt"
	"net/url"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/inspectors/store/config"
)

type StoreTransformsApiInspector struct {
	config config.StoreInspectorConfig
}

func NewStoreTransformsAPIInspector(cfg config.StoreInspectorConfig) *StoreTransformsApiInspector {
	return &StoreTransformsApiInspector{
		config: cfg,
	}
}

func (*StoreTransformsApiInspector) ID() string {
	return "store.transforms-api"
}

func (ins *StoreTransformsApiInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	slog := a.Logger()

	if _, err := config.NewStoreTransformsAPIURLInfo(u, &ins.config, slog); err == nil {
		a.SetRequestPending(ins, "valid URL for store transforms API endpoint")
	}

	return nil // We don't recognize the request
}

type platformInfo struct {
	Name         string `json:"name"`
	Channel      string `json:"channel"`
	Architecture string `json:"architecture"`
}

type revisionInfo struct {
	Platform platformInfo `json:"platform"`
	Revision int          `json:"revision"`
}

type commitInfo struct {
	RemoteURL  string `json:"remote-url"`
	CommitHash string `json:"commit-hash"`
}

type channelInfo struct {
	Name   string `json:"name"`
	Track  string `json:"track"`
	Risk   string `json:"risk"`
	Branch string `json:"branch"`
}

type transformPackage struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

type transformFrom struct {
	Channel channelInfo `json:"channel"`
}

type transformTo struct {
	Channel   channelInfo    `json:"channel"`
	Commit    commitInfo     `json:"commit"`
	Revisions []revisionInfo `json:"revisions"`
}

type transform struct {
	Package transformPackage `json:"package"`
	From    transformFrom    `json:"from"`
	To      transformTo      `json:"to"`
}

type workspaceTransforms struct {
	WorkspaceID string      `json:"workspace-id"`
	Transforms  []transform `json:"transforms"`
}

func (ins *StoreTransformsApiInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/json") {
		return nil
	}

	decoder := json.NewDecoder(f)
	var data workspaceTransforms
	if err := decoder.Decode(&data); err != nil {
		return nil // we don't recognize this artifact
	}

	if data.WorkspaceID == "" || data.Transforms == nil {
		return nil // we don't recognize this artifact
	}

	transforms := make([]string, 0, len(data.Transforms))
	for _, t := range data.Transforms {
		if t.Package.Type != "bin" {
			a.SetResponseRejected(ins, "invalid package type").Annotate(
				Annotation{
					"workspace-id": data.WorkspaceID,
					"package-name": t.Package.Name,
					"package-type": t.Package.Type,
				},
			)
			return nil
		}
		transforms = append(transforms, fmt.Sprintf("%s from %s to %s", t.Package.Name, t.From.Channel.Name, t.To.Channel.Name))
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.StoreTransformsAPI,
		Name:        "Store protocol response",
		Description: "Store response for workspace transforms request",
	})

	a.SetResponseApproved(ins, "valid store transforms API response").Annotate(
		Annotation{
			"workspace-id": data.WorkspaceID,
			"transforms":   transforms,
		},
	)

	return nil
}
