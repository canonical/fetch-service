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

package lxd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
)

type RootfsInspector struct {
}

func NewRootfsInspector() *RootfsInspector {
	return &RootfsInspector{}
}

func (RootfsInspector) ID() string {
	return "lxd.rootfs"
}

// InspectRequest verifies if the request complies with policy.
func (ins *RootfsInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	info, err := NewProductItemUrlInfo(u)
	if err != nil {
		return nil
	}

	a.SetRequestPending(ins, "valid URL for LXD product item").Annotate(
		Annotation{
			"image-series": info.Series,
			"image-date":   info.Date,
			"image-name":   info.Name,
		},
	)
	return nil // we don't recognize this request
}

type RootfsMetadataProperties struct {
	Architecture string `json:"architecture"`
	Description  string `json:"description"`
	Os           string `json:"os"`
	Series       string `json:"series"`
}

type RootfsMetadata struct {
	Architecture string                   `json:"architecture"`
	CreationDate int64                    `json:"creation_date"`
	Properties   RootfsMetadataProperties `json:"properties"`
}

// InspectArtifact extracts metadata from a known artifact file format.
func (ins *RootfsInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/gzip") {
		return nil
	}

	slog := a.Logger()

	zf, err := gzip.NewReader(f)
	if err != nil {
		return nil // We don't recognize this artifact
	}

	var rmd RootfsMetadata
	var hasRootfs bool

	// Read tarball and parse metadata
	tf := tar.NewReader(zf)
	for {
		h, err := tf.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			slog.Debugf("rootfs tar parsing error: %s", err)
			return nil // File may not be a tarball
		}

		if h.Name == "metadata.yaml" {
			slog.Debug("metadata.yaml found in tarball")
			// I know it's weird, but metadata.yaml is actually a json file.
			decoder := json.NewDecoder(tf)
			if err := decoder.Decode(&rmd); err != nil {
				return nil // we don't recognize this artifact
			}
		} else if strings.HasPrefix(h.Name, "rootfs/") {
			slog.Debug("rootfs entry found in tarball")
			hasRootfs = true
			if rmd.Architecture != "" {
				break
			}
		}
	}

	if rmd.Architecture == "" || rmd.Properties.Os == "" || !hasRootfs {
		return nil // we don't recognize this artifact
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:         mimetypes.LxdRootfs,
		Name:         "LXD rootfs image",
		Version:      strconv.FormatInt(rmd.CreationDate, 10),
		Description:  rmd.Properties.Description,
		Architecture: rmd.Properties.Architecture,
	})

	_, ok := a.ResponseStringAnnotation(simpleStreamsDownloadInspectorID, productItemPath)
	if ok {
		a.SetResponseApproved(ins, "valid LXD rootfs tarball").Annotate(
			Annotation{
				"architecture":  rmd.Architecture,
				"creation-date": rmd.CreationDate,
				"os":            rmd.Properties.Os,
				"series":        rmd.Properties.Series,
			},
		)
	} else {
		a.SetResponseRejected(ins, "artifact not verified against product items").Annotate(
			Annotation{
				"architecture":  rmd.Architecture,
				"creation-date": rmd.CreationDate,
				"os":            rmd.Properties.Os,
				"series":        rmd.Properties.Series,
			},
		)
	}

	return nil
}
