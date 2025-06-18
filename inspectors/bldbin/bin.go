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

package bldbin

import (
	"archive/tar"
	"fmt"
	"io"
	"net/url"

	"github.com/xi2/xz"
	"gopkg.in/yaml.v3"

	"github.com/canonical/fetch-service/inspectors/bldbin/config"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
)

// Fields in the bld bin metadata.yaml
type BldBinMetadata struct {
	Name         string `yaml:"name"`
	Version      string `yaml:"version"`
	Base         string `yaml:"base"`
	Architecture string `yaml:"architecture"`
	Summary      string `yaml:"summary"`
	Description  string `yaml:"description"`
	License      string `yaml:"license"`
	Title        string `yaml:"title"`
	Contact      string `yaml:"contact"`
}

type BldBinInspector struct {
	config config.BldBinInspectorConfig
}

func NewBldBinInspector(cfg config.BldBinInspectorConfig) *BldBinInspector {
	return &BldBinInspector{config: cfg}
}

func (BldBinInspector) ID() string {
	return "bld.bin"
}

func (ins *BldBinInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	slog := a.Logger()

	_, err = config.NewBldBinUrlInfo(u, &ins.config, slog)
	if err != nil {
		return nil // We don't recognize the request
	}

	a.SetRequestPending(ins, "valid URL for bin package download")

	return nil
}

func (ins *BldBinInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	xr, err := xz.NewReader(f, 0)
	if err != nil {
		return nil
	}

	slog := a.Logger()

	tf := tar.NewReader(xr)
	metadataFound := false

	for !metadataFound {
		h, err := tf.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		switch h.Name {
		case "./metadata.yaml":
			slog.Debug("bld bin metadata found")
			binmd, err := ReadMetadata(tf)
			if err != nil {
				return err
			}
			if binmd.Name == "" || binmd.Version == "" || binmd.Base == "" || binmd.Architecture == "" {
				return nil // Not our metadata, file format is something else
			}

			revision, ok := a.ResponseStringAnnotation("store.api", "revision")
			if !ok {
				revision = "0"
			}

			md := ArtifactMetadata{
				Type:          mimetypes.BldBinPackage,
				Name:          binmd.Name,
				Version:       binmd.Version,
				Description:   binmd.Summary,
				Architecture:  binmd.Architecture,
				License:       binmd.License,
				Vendor:        binmd.Contact,
				StoreRevision: revision,
			}

			a.SetArtifactMetadata(md)
			a.SetResponseApproved(ins, "bin package metadata parsed")

			metadataFound = true
		}
	}

	return nil
}

// ReadMetadata reads and decodes the bin metadata.yaml file.
func ReadMetadata(f io.Reader) (*BldBinMetadata, error) {
	var binmd BldBinMetadata
	decoder := yaml.NewDecoder(f)

	err := decoder.Decode(&binmd)
	if err != nil {
		return nil, err
	}

	return &binmd, nil
}
