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
	"encoding/json"
	"fmt"
	"regexp"
	"sync"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
)

var (
	downloadRequestURL      = regexp.MustCompile(`^https://cloud-images.ubuntu.com:443/([\w-\/]+)/streams/v1/([\w-\.\/:]+):download.json$`)
	downloadImageRequestURL = regexp.MustCompile(`^https://cloud-images.ubuntu.com:443/[\w-]+/[\w-]+/([\w-\/\.]+)$`)
)

const (
	productItemPath   = "product-item-path"
	productItemSha256 = "product-item-sha256"
	productItems      = "product-items"
)

type SimpleStreamsDownloadInspector struct {
	productItems     map[string]string
	productItemsLock sync.Mutex
}

func NewSimpleStreamsDownloadInspector() *SimpleStreamsDownloadInspector {
	return &SimpleStreamsDownloadInspector{}
}

func (ins *SimpleStreamsDownloadInspector) ID() string {
	return "lxd.simple-streams.download"
}

func (ins *SimpleStreamsDownloadInspector) matchItemFromURL(downloadURL string) (string, error) {
	// Return early if no items computed
	if ins.productItems == nil {
		return "", fmt.Errorf("no items")
	}

	m := downloadImageRequestURL.FindStringSubmatch(downloadURL)
	if len(m) > 1 {
		if _, ok := ins.productItems[m[1]]; ok {
			return m[1], nil
		} else {
			return m[1], fmt.Errorf("no item matching %s", downloadURL)
		}
	}

	return "", fmt.Errorf("no item matching %s", downloadURL)
}

func (ins *SimpleStreamsDownloadInspector) InspectRequest(a RequestArtifact) error {
	slog := a.Logger()

	m := downloadRequestURL.FindStringSubmatch(a.DownloadURL())
	if len(m) > 1 {
		// Annotate the stream as it comes from cloud images
		a.SetRequestPending(ins, "valid Simple Streams Download URL").Annotate(
			Annotation{
				"match":  downloadRequestURL,
				"stream": m[1],
			},
		)
		return nil
	}

	item, err := ins.matchItemFromURL(a.DownloadURL())
	if err != nil {
		slog.Debug(err)
		return nil
	}

	a.SetRequestPending(ins, "valid Simple Streams Download URL").Annotate(
		Annotation{
			"match":         downloadImageRequestURL,
			productItemPath: item,
		},
	)

	return nil

}

type simpleStreamsDownload struct {
	Updated   string                                `json:"updated"`
	Format    string                                `json:"format"`
	Datatype  string                                `json:"datatype"`
	ContentId string                                `json:"content_id"`
	License   string                                `json:"license"`
	Creator   string                                `json:"Creator_id"`
	Products  map[string]simpleStreamProductEntries `json:"products"`
}

type simpleStreamProductEntries struct {
	Aliases         string                                 `json:"aliases"`
	Arch            string                                 `json:"arch"`
	Os              string                                 `json:"os"`
	Release         string                                 `json:"release"`
	ReleaseCodename string                                 `json:"release_codename"`
	ReleaseTitle    string                                 `json:"release_title"`
	SupportEol      string                                 `json:"support_eol"`
	Supported       bool                                   `json:"supported"`
	Version         string                                 `json:"version"`
	Versions        map[string]simpleStreamsProductVersion `json:"versions"`
}

type simpleStreamsProductVersion struct {
	Label   string                              `json:"label"`
	PubName string                              `json:"pubname"`
	Items   map[string]simpleStreamsProductItem `json:"items"`
}

type simpleStreamsProductItem struct {
	FType  string `json:"ftype"`
	Path   string `json:"path"`
	Sha256 string `json:"sha256"`
	Size   uint32 `json:"size"`
}

func (ins *SimpleStreamsDownloadInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	slog := a.Logger()

	if !a.MimetypeIs("application/json") {
		itemPath, err := ins.matchItemFromURL(a.DownloadURL())
		if err != nil {
			slog.Debug(err)
		}
		if itemPath != "" {
			return ins.inspectProductItem(a, itemPath)
		}
		return nil
	}

	stream, ok := a.RequestStringAnnotation(ins.ID(), "stream")
	if !ok {
		return nil
	}
	slog.Debugf("parsing Simple Streams Download for stream %s", stream)

	decoder := json.NewDecoder(f)
	var b simpleStreamsDownload
	if err := decoder.Decode(&b); err != nil {
		slog.Debug(err)
		return nil // we don't recognize this artifact
	}

	if b.Format != productFormat {
		slog.Debugf("unsupported format when parsing Simple Streams Download file %s", b.Format)
		return nil
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.SimpleStreams,
		Name:        "Simple Streams Download",
		Description: fmt.Sprintf("Simple Streams Download for %s", b.ContentId),
	})

	a.SetResponseApproved(ins, "valid Simple Streams Download file").Annotate(
		Annotation{productItems: ins.extractSupportedUbuntuImages(b.Products)},
	)

	return nil
}

func (ins *SimpleStreamsDownloadInspector) inspectProductItem(a ResponseArtifact, itemPath string) error {
	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.SimpleStreamsProduct,
		Name:        "Simple Streams Product Item",
		Description: fmt.Sprintf("Simple Streams Product item from %s", itemPath),
	})

	sha256, ok := ins.productItems[itemPath]

	if !ok {
		a.SetResponseRejected(ins, "sha256 missing for item").Annotate(Annotation{
			productItemPath:   itemPath,
			productItemSha256: a.Sha256().String(),
		})

		return nil
	}

	if sha256 != a.Sha256().String() {
		a.SetResponseRejected(ins, "sha256 mismatch").Annotate(Annotation{
			"expected-sha256": sha256,
			productItemSha256: a.Sha256().String(),
		})

		return nil

	}

	a.SetResponseApproved(ins, "valid Simple Streams Product item").Annotate(Annotation{
		productItemPath: itemPath,
		"sha256":        a.Sha256().String(),
	})

	return nil
}

// extractSupportedUbuntuImages creates a map of image paths to SHA256 values
// for supported Ubuntu versions (24.04, 22.04, 20.04) from product entries
func (ins *SimpleStreamsDownloadInspector) extractSupportedUbuntuImages(products map[string]simpleStreamProductEntries) map[string]string {
	ins.productItemsLock.Lock()
	defer ins.productItemsLock.Unlock()

	result := make(map[string]string)
	supportedVersions := map[string]bool{
		"24.04": true,
		"22.04": true,
		"20.04": true,
	}

	for _, product := range products {
		if !product.Supported {
			continue
		}

		if !supportedVersions[product.Version] {
			continue
		}

		for _, version := range product.Versions {
			for _, item := range version.Items {
				if item.Path != "" && item.Sha256 != "" {
					result[item.Path] = item.Sha256
				}
			}
		}
	}

	if ins.productItems == nil {
		ins.productItems = result
	} else {
		for k, v := range result {
			ins.productItems[k] = v
		}
	}

	return result
}
