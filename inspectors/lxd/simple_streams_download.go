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
	"maps"
	"net/url"
	"regexp"
	"strings"
	"sync"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
)

var downloadImageRequestURL = regexp.MustCompile(`^/[\w-]+/[\w-]+/([\w-\/\.]+)$`)

const (
	simpleStreamsDownloadInspectorID = "lxd.simple-streams.download"

	productItemPath   = "product-item-path"
	productItemSha256 = "product-item-sha256"
	productItems      = "product-items"
)

type SimpleStreamsDownloadInspector struct {
	productItems     map[string]string // Map image file name to sha256 digest
	productItemsLock sync.Mutex
}

func NewSimpleStreamsDownloadInspector() *SimpleStreamsDownloadInspector {
	return &SimpleStreamsDownloadInspector{}
}

func (ins *SimpleStreamsDownloadInspector) ID() string {
	return simpleStreamsDownloadInspectorID
}

func (ins *SimpleStreamsDownloadInspector) matchItemFromURL(downloadURL string) (string, error) {
	u, err := url.Parse(downloadURL)
	if err != nil {
		return "", fmt.Errorf("cannot parse URL: %s", err)
	}

	ins.productItemsLock.Lock()
	defer ins.productItemsLock.Unlock()

	// Return early if no items computed
	if ins.productItems == nil {
		return "", fmt.Errorf("no items")
	}

	m := downloadImageRequestURL.FindStringSubmatch(u.Path)
	if len(m) > 1 {
		if _, ok := ins.productItems[m[1]]; !ok {
			return m[1], fmt.Errorf("no item matching %s", downloadURL)
		}

		return m[1], nil
	}

	return "", fmt.Errorf("no item matching %s", downloadURL)
}

func (ins *SimpleStreamsDownloadInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	info, err := NewSimpleStreamsDownloadURLInfo(u)
	if err == nil {
		a.SetRequestPending(ins, "valid Simple Streams download request URL").Annotate(
			Annotation{
				"stream":        info.Stream,
				productItemPath: info.ItemPath,
			},
		)
		return nil
	}

	pinfo, err := NewProductItemURLInfo(u)
	if err == nil {
		a.SetRequestPending(ins, "valid Simple Streams product item URL").Annotate(
			Annotation{
				"product-name": pinfo.Name,
			},
		)
		return nil
	}

	return nil
}

type simpleStreamsDownload struct {
	Updated   string                                `json:"updated"`
	Format    string                                `json:"format"`
	Datatype  string                                `json:"datatype"`
	ContentID string                                `json:"content_id"`
	License   string                                `json:"license"`
	Creator   string                                `json:"creator"`
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
	sl := a.Logger()

	if a.MimetypeIs("application/gzip") {
		// Check if this is a product item.
		// State was set when the download inspector ran,
		itemPath, err := ins.matchItemFromURL(a.DownloadURL())
		if err != nil {
			sl.Debug(err)
		}
		if itemPath != "" {
			return ins.inspectProductItem(a, itemPath)
		}
		return nil
	}

	if !a.MimetypeIs("application/json") {
		return nil // We don't recognize this artifact
	}

	decoder := json.NewDecoder(f)
	var dl simpleStreamsDownload
	if err := decoder.Decode(&dl); err != nil {
		sl.Debug(err)
		return nil // we don't recognize this artifact
	}

	if dl.Format != productFormat || dl.Datatype == "" || dl.Updated == "" || dl.Products == nil {
		return nil // we don't recognize this format
	}

	stream, ok := a.RequestStringAnnotation(ins.ID(), "stream")
	if !ok {
		// The request inspector must have set a "stream" annotation.
		return fmt.Errorf("missing stream in request annotations")
	}
	sl.Debugf("parsed Simple Streams Download for stream %s", stream)

	md := ArtifactMetadata{
		Type:        mimetypes.SimpleStreamsProducts,
		Name:        "Simple Streams Download",
		Description: fmt.Sprintf("Simple Streams Download for %s", dl.ContentID),
	}

	a.SetResponseApproved(ins, "valid Simple Streams Download file", md).Annotate(
		Annotation{productItems: ins.extractSupportedUbuntuImages(dl.Products)},
	)

	return nil
}

func (ins *SimpleStreamsDownloadInspector) getProductItemDigest(itemPath string) (string, bool) {
	ins.productItemsLock.Lock()
	defer ins.productItemsLock.Unlock()

	digest, ok := ins.productItems[itemPath]
	return digest, ok
}

func (ins *SimpleStreamsDownloadInspector) inspectProductItem(a ResponseArtifact, itemPath string) error {
	md := ArtifactMetadata{} // We don't know what this artifact is
	digest, ok := ins.getProductItemDigest(itemPath)
	if !ok {
		a.SetResponseRejected(ins, "sha256 missing for item", md).Annotate(Annotation{
			productItemPath:   itemPath,
			productItemSha256: a.Sha256().String(),
		})
		return nil
	}

	if digest != a.Sha256().String() {
		a.SetResponseRejected(ins, "sha256 mismatch", md).Annotate(Annotation{
			"expected-sha256": digest,
			productItemSha256: a.Sha256().String(),
		})
		return nil

	}

	a.SetResponseUnknown(ins, "simple streams product item matches digest", NoMetadata).Annotate(Annotation{
		productItemPath: itemPath,
		"sha256":        a.Sha256().String(),
	})

	return nil
}

// extractSupportedUbuntuImages creates a map of image paths to SHA256 values
// for supported Ubuntu versions from product entries.
func (ins *SimpleStreamsDownloadInspector) extractSupportedUbuntuImages(products map[string]simpleStreamProductEntries) map[string]string {
	result := make(map[string]string)
	supportedVersions := map[string]struct{}{
		"26.10": {},
		"26.04": {},
		"24.04": {},
		"22.04": {},
		"20.04": {},
	}

	for _, product := range products {
		if _, ok := supportedVersions[product.Version]; !ok {
			continue
		}

		for _, version := range product.Versions {
			for _, item := range version.Items {
				if !strings.HasSuffix(item.Path, ".tar.gz") {
					continue
				}
				if item.Path != "" && item.Sha256 != "" {
					result[item.Path] = item.Sha256
				}
			}
		}
	}

	ins.productItemsLock.Lock()
	defer ins.productItemsLock.Unlock()

	if ins.productItems == nil {
		ins.productItems = result
	} else {
		maps.Copy(ins.productItems, result)
	}

	return result
}
