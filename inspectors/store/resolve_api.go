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
	"slices"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/inspectors/store/config"
)

type StoreResolveAPIInspector struct {
	config config.StoreInspectorConfig
}

func NewStoreResolveAPIInspector(cfg config.StoreInspectorConfig) *StoreResolveAPIInspector {
	return &StoreResolveAPIInspector{
		config: cfg,
	}
}

func (*StoreResolveAPIInspector) ID() string {
	return "store.resolve-api"
}

func (ins *StoreResolveAPIInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	sl := a.Logger()

	if _, err := config.NewStoreResolveAPIURLInfo(u, &ins.config, sl); err == nil {
		a.SetRequestPending(ins, "valid URL for store resolve API endpoint")
	}

	return nil // We don't recognize the request
}

type storeResolveAPIResult struct {
	ID          string `json:"id"`
	InstanceKey string `json:"instance-key"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Status      string `json:"status"`
}

type storeResolveAPIData struct {
	CraftResults   []storeResolveAPIResult `json:"craft-results"`
	PackageResults []storeResolveAPIResult `json:"package-results"`
}

func (ins *StoreResolveAPIInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/json") {
		return nil
	}

	decoder := json.NewDecoder(f)
	var data storeResolveAPIData
	if err := decoder.Decode(&data); err != nil {
		return nil // we don't recognize this artifact
	}

	if len(data.CraftResults) == 0 && len(data.PackageResults) == 0 {
		return nil // we don't recognize this artifact
	}

	var result storeResolveAPIResult
	if len(data.CraftResults) > 0 {
		result = data.CraftResults[0]
	} else {
		result = data.PackageResults[0]
	}

	if result.Namespace == "" {
		return nil // we don't recognize this artifact
	}

	if result.Status != "ok" && result.Status != "error" {
		return nil // we don't recognize this artifact
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.StoreResolveAPI,
		Name:        "Store protocol response",
		Description: "Store response for resolve_revisions request",
	})

	listedCrafts := make([]string, len(data.CraftResults))
	for i, item := range data.CraftResults {
		listedCrafts[i] = item.Name
	}

	listedPackages := make([]string, len(data.PackageResults))
	for i, item := range data.PackageResults {
		listedPackages[i] = item.Name
	}

	notes := Annotation{
		"resolved-craft-list":   listedCrafts,
		"resolved-package-list": listedPackages,
		"instance-key":          result.InstanceKey,
	}

	validNamespaces := []string{"bin", "charm", "rock", "snap"}
	if !slices.Contains(validNamespaces, result.Namespace) {
		notes["namespace"] = result.Namespace
		a.SetResponseRejected(ins, "invalid namespace").Annotate(notes)
		return nil
	}

	a.SetResponseApproved(ins, "valid store resolve_revisions API response").Annotate(notes)

	return nil
}
