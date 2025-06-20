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
	"archive/tar"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"sync"

	"github.com/xi2/xz"
	"golang.org/x/crypto/sha3"

	"github.com/canonical/fetch-service/inspectors/bldbin"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/inspectors/store/config"
)

type storeApiRevisionInfo struct {
	Sha3_384 string // SHA3-384
	Size     uint64 // File size
	Revision string // Revision number
	Channel  string // Channel name
}

type storeApiInfo struct {
	Type      string // Package type
	ID        string // Package ID
	Publisher string // Package publisher
	RevInfo   map[string]storeApiRevisionInfo
}

type StoreApiInspector struct {
	config  config.StoreInspectorConfig
	ids     map[string]*storeApiInfo // Map from IDs to file information
	idsLock sync.Mutex
}

func NewStoreApiInspector(cfg config.StoreInspectorConfig) *StoreApiInspector {
	return &StoreApiInspector{
		config: cfg,
		ids:    map[string]*storeApiInfo{},
	}
}

func (ins *StoreApiInspector) setStoreApiInfo(pkgid string, ainfo *storeApiInfo) {
	ins.idsLock.Lock()
	defer ins.idsLock.Unlock()

	ins.ids[pkgid] = ainfo
}

func (ins *StoreApiInspector) findStoreApiInfo(sha3_384 string) (*storeApiInfo, string, string) {
	ins.idsLock.Lock()
	defer ins.idsLock.Unlock()

	for _, info := range ins.ids {
		rinfo, ok := info.RevInfo[sha3_384]
		if ok {
			return info, rinfo.Revision, rinfo.Channel
		}
	}
	return nil, "0", ""
}

func (*StoreApiInspector) ID() string {
	return "store.api"
}

func (ins *StoreApiInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	slog := a.Logger()

	info, err := config.NewStoreApiUrlInfo(u, &ins.config, slog)
	if err != nil {
		return nil // We don't recognize the request
	}

	a.SetRequestPending(ins, "valid URL for store API endpoint").Annotate(
		Annotation{
			"type":    info.PackageType,
			"package": info.PackageName,
		},
	)

	return nil
}

type RevisionDownload struct {
	Sha3_384 string `json:"sha3-384"`
	Size     int    `json:"size"`
	Url      string `json:"url"`
}

type apiInfoChannelMapRevision struct {
	Download RevisionDownload `json:"download"`
	Revision int              `json:"revision"`
}

type apiInfoChannelMapChannel struct {
	Name string `json:"name"`
}

type apiInfoChannelMap struct {
	Channel  apiInfoChannelMapChannel  `json:"channel"`
	Revision apiInfoChannelMapRevision `json:"revision"`
}

type apiInfoMetadataPublisher struct {
	DisplayName string `json:"display-name"`
	ID          string `json:"id"`
	Username    string `json:"username"`
}

type apiInfoMetadata struct {
	Contact     string                   `json:"contact"`
	Description string                   `json:"description"`
	License     string                   `json:"license"`
	Summary     string                   `json:"summary"`
	Publisher   apiInfoMetadataPublisher `json:"publisher"`
}

type apiInfo struct {
	ChannelMap   []apiInfoChannelMap `json:"channel-map"`
	DefaultTrack string              `json:"default-track"`
	Metadata     apiInfoMetadata     `json:"metadata"`
	Name         string              `json:"name"`
	PackageID    string              `json:"package-id"`
}

func (ins *StoreApiInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if a.MimetypeIs("application/x-xz") { // May be a bld bin
		return ins.validateBldBin(f, a)
	}

	if !a.MimetypeIs("application/json") {
		return nil
	}

	slog := a.Logger()

	decoder := json.NewDecoder(f)
	var info apiInfo
	if err := decoder.Decode(&info); err != nil {
		return nil // we don't recognize this artifact
	}

	if len(info.ChannelMap) == 0 {
		slog.Debug("no channel information")
		return nil // we don't recognize this artifact
	}

	if info.ChannelMap[0].Revision.Download.Url == "" {
		slog.Debug("no download url")
		return nil // we don't recognize this artifact
	}

	if info.Metadata.Publisher.Username == "" {
		slog.Debug("no publisher username")
		return nil // we don't recognize this artifact
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.StoreAPI,
		Name:        "Store protocol response",
		Description: "Store response for info request",
	})

	if info.Name == "" || info.PackageID == "" {
		a.SetResponseRejected(ins, "package name or ID are not set")
		return nil
	}

	pkgType, ok := a.RequestStringAnnotation(ins.ID(), "type")
	if !ok {
		a.SetResponseRejected(ins, "package type request annotation not found")
		return nil
	}

	a.SetResponseApproved(ins, "valid bin store API info endpoint response").Annotate(
		Annotation{
			"name":       info.Name,
			"type":       pkgType,
			"package-id": info.PackageID,
			"publisher":  info.Metadata.Publisher.DisplayName,
		})

	ainfo := &storeApiInfo{
		Type:      pkgType,
		ID:        info.PackageID,
		Publisher: info.Metadata.Publisher.DisplayName,
		RevInfo:   map[string]storeApiRevisionInfo{},
	}

	for _, cinfo := range info.ChannelMap {
		sha3_384 := cinfo.Revision.Download.Sha3_384
		ainfo.RevInfo[sha3_384] = storeApiRevisionInfo{
			Sha3_384: sha3_384,
			Size:     uint64(cinfo.Revision.Download.Size),
			Revision: strconv.Itoa(cinfo.Revision.Revision),
			Channel:  cinfo.Channel.Name,
		}

	}

	ins.setStoreApiInfo(info.PackageID, ainfo)

	return nil
}

func (ins *StoreApiInspector) validateBldBin(f ArtifactReader, a ResponseArtifact) error {
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
			binmd, err := bldbin.ReadMetadata(tf)
			if err != nil {
				return err
			}
			if binmd.Name == "" || binmd.Version == "" || binmd.Base == "" || binmd.Architecture == "" {
				return nil // Not our metadata, file format is something else
			}

			// Get sha3-384 digest and retrieve store lookup state. Can't verify URL
			// because the artifact download location is redirected.
			sha3_384, err := sha3_384Digest(f)
			if err != nil {
				return err
			}

			ainfo, rev, channel := ins.findStoreApiInfo(sha3_384)
			if ainfo != nil {
				if ainfo.Type == "bins" {
					// Setting as Unknown to avoid approval in case the bld bin inspector
					// doesn't recognize the format.
					a.SetResponseUnknown(ins, "file digest matches store API bin request").Annotate(
						Annotation{
							"package-id": ainfo.ID,
							"revision":   rev,
							"digest":     sha3_384,
							"channel":    channel,
						},
					)

				} else {
					a.SetResponseRejected(ins, "file digest matches a request for a different package type").Annotate(
						Annotation{
							"package-id": ainfo.ID,
							"type":       ainfo.Type,
							"digest":     sha3_384,
							"channel":    channel,
						},
					)
				}
			} else {
				a.SetResponseRejected(ins, "file digest does not match any store API request")
			}

			metadataFound = true
		}
	}

	return nil
}

func sha3_384Digest(f io.ReadSeeker) (string, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	hash := sha3.New384()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}

	hashBytes := hash.Sum(nil)
	return hex.EncodeToString(hashBytes), nil
}
