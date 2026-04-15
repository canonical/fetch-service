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
	"fmt"
	"image/png"
	"net/url"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/inspectors/store/config"
)

type StoreAppMediaInspector struct {
	config config.StoreInspectorConfig
}

func NewStoreAppMediaInspector(cfg config.StoreInspectorConfig) *StoreAppMediaInspector {
	return &StoreAppMediaInspector{
		config: cfg,
	}
}

func (*StoreAppMediaInspector) ID() string {
	return "store.appmedia"
}

func (ins *StoreAppMediaInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	sl := a.Logger()

	if _, err := config.NewStoreAppMediaURLInfo(u, &ins.config, sl); err == nil {
		a.SetRequestPending(ins, "valid URL for store app media")
	}

	return nil
}

func (ins *StoreAppMediaInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if a.MimetypeIs("image/png") {
		return ins.inspectArtifactPNG(f, a)
	}

	return nil
}

func (ins *StoreAppMediaInspector) inspectArtifactPNG(f ArtifactReader, a ResponseArtifact) error {
	sl := a.Logger()

	cfg, err := png.DecodeConfig(f)
	if err != nil {
		sl.Infof("cannot decode PNG file: %s", err)
		return nil
	}

	notes := Annotation{
		"height": cfg.Height,
		"width":  cfg.Width,
	}

	md := ArtifactMetadata{
		Type:        mimetypes.StoreAppmediaPNG,
		Name:        "Image file",
		Description: "Store media file in PNG format",
	}

	a.SetArtifactMetadata(md)

	if a.InspectorRequestOpinionPending(ins) {
		a.SetResponseApproved(ins, "store media file in PNG format").Annotate(notes)
	} else {
		a.SetResponseUnknown(ins, "unknown PNG image").Annotate(notes)
	}

	return nil
}
