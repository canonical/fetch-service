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
	bconfig "github.com/canonical/fetch-service/inspectors/bldbin/config"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/inspectors/store/config"
)

type storeInfoApiRevisionInfo struct {
	Sha3_384 string // SHA3-384
	Size     uint64 // File size
	Revision string // Revision number
	Channel  string // Channel name
}

type storeInfoApiInfo struct {
	Type      string // Package type
	ID        string // Package ID
	Publisher string // Package publisher
	RevInfo   map[string]storeInfoApiRevisionInfo
}

type StoreInfoApiInspector struct {
	config  config.StoreInspectorConfig
	bconfig bconfig.BldBinInspectorConfig
	ids     map[string]*storeInfoApiInfo // Map from IDs to file information
	idsLock sync.Mutex
}

func NewStoreInfoApiInspector(cfg config.StoreInspectorConfig, bcfg bconfig.BldBinInspectorConfig) *StoreInfoApiInspector {
	return &StoreInfoApiInspector{
		config:  cfg,
		bconfig: bcfg,
		ids:     map[string]*storeInfoApiInfo{},
	}
}

func (ins *StoreInfoApiInspector) setInfo(pkgid string, ainfo *storeInfoApiInfo) {
	ins.idsLock.Lock()
	defer ins.idsLock.Unlock()

	ins.ids[pkgid] = ainfo
}

func (ins *StoreInfoApiInspector) findInfo(sha3_384 string) (*storeInfoApiInfo, string, string) {
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

func (*StoreInfoApiInspector) ID() string {
	return "store.info-api"
}

func (ins *StoreInfoApiInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	slog := a.Logger()

	if info, err := config.NewStoreInfoApiUrlInfo(u, &ins.config, slog); err == nil {
		a.SetRequestPending(ins, "valid URL for store info API endpoint").Annotate(
			Annotation{
				"type":    info.PackageType,
				"package": info.PackageName,
			},
		)
	} else if _, err := bconfig.NewBldBinUrlInfo(u, &ins.bconfig, slog); err == nil {
		a.SetRequestPending(ins, "valid URL for bld bin download")
	}

	return nil // We don't recognize the request
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

func (ins *StoreInfoApiInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
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

	if info.Name == "" || info.PackageID == "" {
		slog.Debug("no name or package ID")
		return nil // we don't recognize this artifact
	}

	pkgType, ok := a.RequestStringAnnotation(ins.ID(), "type")
	if !ok {
		slog.Debug("no package type annotation")
		return nil // we don't recognize this artifact
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.StoreInfoAPI,
		Name:        "Store protocol response",
		Description: "Store response for info request",
	})

	a.SetResponseApproved(ins, "valid store info API response").Annotate(
		Annotation{
			"name":       info.Name,
			"type":       pkgType,
			"package-id": info.PackageID,
			"publisher":  info.Metadata.Publisher.DisplayName,
		})

	ainfo := &storeInfoApiInfo{
		Type:      pkgType,
		ID:        info.PackageID,
		Publisher: info.Metadata.Publisher.DisplayName,
		RevInfo:   map[string]storeInfoApiRevisionInfo{},
	}

	for _, cinfo := range info.ChannelMap {
		sha3_384 := cinfo.Revision.Download.Sha3_384
		ainfo.RevInfo[sha3_384] = storeInfoApiRevisionInfo{
			Sha3_384: sha3_384,
			Size:     uint64(cinfo.Revision.Download.Size),
			Revision: strconv.Itoa(cinfo.Revision.Revision),
			Channel:  cinfo.Channel.Name,
		}

	}

	ins.setInfo(info.PackageID, ainfo)

	return nil
}

func (ins *StoreInfoApiInspector) validateBldBin(f ArtifactReader, a ResponseArtifact) error {
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
			return nil // We don't recognize this artifact (not a tarball)
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

			ainfo, rev, channel := ins.findInfo(sha3_384)
			if ainfo != nil {
				if ainfo.Type == "bins" {
					// Setting as Unknown to avoid approval in case the bld bin inspector
					// doesn't recognize the format.
					a.SetResponseUnknown(ins, "file digest matches store info API bin request").Annotate(
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
				a.SetResponseRejected(ins, "file digest does not match any store info API request")
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
