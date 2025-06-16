// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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

package oci

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sync"

	"github.com/opencontainers/go-digest"
	spec "github.com/opencontainers/image-spec/specs-go/v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/oci/config"
)

type OciInspector struct {
	config config.OciInspectorConfig

	// Image pull requests. Mapped from digest to info.
	pulls     map[string]imagePullInfo
	pullsLock sync.Mutex
}

type imagePullInfo struct {
	image string   // Image name.
	url   *url.URL // Request URL.

	// There are two kinds of image pull requests per the OCI distribution
	// spec[^1]:
	//  - Manifest: GET /v2/<name>/manifests/<reference>
	//  - Blob:     GET /v2/<name>/blobs/<digest>
	//
	// <reference> can either be a tag or a digest.
	//
	// [^1] https://github.com/opencontainers/distribution-spec/blob/main/spec.md#endpoints
	reference string

	spec.Descriptor
}

func NewOciInspector(cfg config.OciInspectorConfig) *OciInspector {
	return &OciInspector{
		config: cfg,
		pulls:  make(map[string]imagePullInfo),
	}
}

func (ins *OciInspector) ID() string {
	return "oci"
}

func (ins *OciInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %w", err)
	}

	if notes, err := ins.newRegistryPing(u); err == nil {
		a.SetRequestPending(ins, "valid registry ping").Annotate(notes)
		return nil
	}

	if notes, err := ins.newRegistryAuth(u); err == nil {
		a.SetRequestPending(ins, "valid registry auth request").Annotate(notes)
		return nil
	}

	if notes, err := ins.newManifestPull(u); err == nil {
		a.SetRequestPending(ins, "valid manifest pull request").Annotate(notes)
		return nil
	}

	if notes, err := ins.newBlobPull(u); err == nil {
		a.SetRequestPending(ins, "valid blob pull request").Annotate(notes)
		return nil
	}

	return nil
}

func (ins *OciInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	// Artifacts should have an associated "request-type" for this inspector.
	reqType, ok := a.RequestStringAnnotation(ins.ID(), "request-type")
	if !ok {
		return nil
	}

	switch reqType {
	case reqTypePing:
		// Ping requests should have an empty body.
		if a.Size() != 0 {
			return nil
		}
		a.SetResponseApproved(ins, "valid registry ping artifact")
	case reqTypeAuth:
		if !a.MimetypeIs("application/json") {
			return nil
		}
		a.SetResponseApproved(ins, "valid registry authentication artifact")
	case reqTypeManifest:
		if a.MimetypeIs(spec.MediaTypeImageIndex) {
			return ins.validateImageIndex(f, a)
		}
		if a.MimetypeIs(spec.MediaTypeImageManifest) {
			return ins.validateImageManifest(f, a)
		}
	case reqTypeBlob:
		return ins.validateImageBlob(f, a)
	}

	return nil
}

const (
	reqTypePing     = "ping"
	reqTypeAuth     = "auth"
	reqTypeManifest = "manifest"
	reqTypeBlob     = "blob"
)

// newRegistryPing checks whether the request matches the "/v2/" endpoint.
// See [end-1] in the OCI distribution spec endpoints.
// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#endpoints
func (ins *OciInspector) newRegistryPing(u *url.URL) (Annotation, error) {
	exp := regexp.MustCompile(`^(.*)/v2/$`)
	match := exp.FindStringSubmatch(u.String())
	if len(match) != 2 {
		return nil, fmt.Errorf("unrecognized request: %s", u)
	}

	regUrl := match[1]
	regName, ok := registryIsAllowed(regUrl, &ins.config)
	if !ok {
		return nil, fmt.Errorf("unrecognized registry: %s", regUrl)
	}

	notes := Annotation{
		"request-type":  reqTypePing,
		"registry-name": regName,
		"registry-url":  regUrl,
	}
	return notes, nil
}

// newRegistryAuth checks if the request matches the autentication URL of any
// configured registries.
func (ins *OciInspector) newRegistryAuth(u *url.URL) (Annotation, error) {
	for name, reg := range ins.config.Registries {
		if reg.AuthUrl.G.Match(u.String()) {
			notes := Annotation{
				"request-type":  reqTypeAuth,
				"registry-name": name,
			}
			return notes, nil
		}
	}
	return nil, fmt.Errorf("unrecognized authentication URL: %s", u)
}

// newManifestPull checks if the request matches the [end-3] endpoint of the
// OCI distribution spec[^1]:
//
//	/v2/<name>/manifests/<reference>
//
// [^1] https://github.com/opencontainers/distribution-spec/blob/main/spec.md#endpoints
func (ins *OciInspector) newManifestPull(u *url.URL) (Annotation, error) {
	return ins.newPull(reqTypeManifest, u)
}

// newBlobPull checks if the request matches the [end-2] endpoint of the
// OCI distribution spec[^1]:
//
//	/v2/<name>/blobs/<digest>
//
// [^1] https://github.com/opencontainers/distribution-spec/blob/main/spec.md#endpoints
func (ins *OciInspector) newBlobPull(u *url.URL) (Annotation, error) {
	return ins.newPull(reqTypeBlob, u)
}

var (
	// Image name regex per the OCI distribution spec:
	// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-manifests
	imageName = regexp.MustCompile(`[a-z0-9]+(?:(?:\.|_|__|-+)[a-z0-9]+)*(?:\/[a-z0-9]+(?:(?:\.|_|__|-+)[a-z0-9]+)*)*`)

	// Image tag regex per the OCI distribution spec:
	// https://github.com/opencontainers/distribution-spec/blob/main/spec.md#pulling-manifests
	imageTag = regexp.MustCompile(`[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}`)

	// Image digest regex per the OCI image spec:
	// https://github.com/opencontainers/image-spec/blob/main/descriptor.md#digests
	imageRef = digest.DigestRegexp
)

// Checks if it is a manifest or a blob pull request.
func (ins *OciInspector) newPull(reqType string, u *url.URL) (Annotation, error) {
	var pattern *regexp.Regexp
	switch reqType {
	case reqTypeManifest:
		// See "end-3" endpoint in the OCI distribution spec.
		pattern = regexp.MustCompile(fmt.Sprintf(
			"^(.*)/v2/(%s)/manifests/((?:%s)|(?:%s))$",
			imageName.String(), imageRef.String(), imageTag.String()))
	case reqTypeBlob:
		// See "end-2" endpoint in the OCI distribution spec.
		pattern = regexp.MustCompile(fmt.Sprintf(
			"^(.*)/v2/(%s)/blobs/(%s)$",
			imageName.String(), imageRef.String()))
	default:
		return nil, fmt.Errorf("unknown request type: %s", reqType)
	}

	match := pattern.FindStringSubmatch(u.String())
	if len(match) != 4 {
		return nil, fmt.Errorf("invalid manifest URL: %s", u)
	}

	regUrl, image, ref := match[1], match[2], match[3]
	cfgName, ok := registryIsAllowed(regUrl, &ins.config)
	if !ok {
		return nil, fmt.Errorf("invalid registry URL: %s", regUrl)
	}

	note := Annotation{
		"request-type":  reqType,
		"registry-name": cfgName,
		"registry-url":  regUrl,
		"image-name":    image,
		"reference":     ref,
	}
	if digest.DigestRegexpAnchored.MatchString(ref) {
		info, ok := ins.getPullInfo(ref)
		if !ok {
			return nil, fmt.Errorf("unknown digest: %s", ref)
		}
		if info.image == "" {
			info.image = image
		}
		if info.url == nil {
			info.url = u
		}
		if info.reference == "" {
			info.reference = ref
		}
		if info.Digest == "" {
			info.Digest = digest.Digest(ref)
		}
		note["digest"] = ref
		ins.addPullInfo(ref, info)
	}
	return note, nil
}

func (ins *OciInspector) addPullInfo(digest string, info imagePullInfo) {
	ins.pullsLock.Lock()
	defer ins.pullsLock.Unlock()

	ins.pulls[digest] = info
}

func (ins *OciInspector) getPullInfo(digest string) (imagePullInfo, bool) {
	ins.pullsLock.Lock()
	defer ins.pullsLock.Unlock()

	info, ok := ins.pulls[digest]
	return info, ok
}

// validateImageIndex checks whether the artifact is truly an OCI image index.
func (ins *OciInspector) validateImageIndex(f ArtifactReader, a ResponseArtifact) error {
	index, err := parseOciImageIndex(f)
	if err != nil {
		return err
	}

	info, meta, notes := ins.extractImageIndexInfo(index, a)
	if err := checkArtifactDigest(info.Digest, f, a); err != nil {
		return err
	}

	ins.addPullInfo(info.Digest.String(), info)
	for _, mf := range index.Manifests {
		mfInfo := imagePullInfo{
			Descriptor: mf,
		}
		ins.addPullInfo(mf.Digest.String(), mfInfo)
	}

	a.SetArtifactMetadata(meta)
	a.SetResponseApproved(ins, "valid OCI image index").Annotate(notes)
	return nil
}

// validateImageIndex checks whether the artifact is truly an OCI image manifest.
func (ins *OciInspector) validateImageManifest(f ArtifactReader, a ResponseArtifact) error {
	manifest, err := parseOciImageManifest(f)
	if err != nil {
		return err
	}

	info, meta, notes := ins.extractImageManifestInfo(manifest, a)
	if err := checkArtifactDigest(info.Digest, f, a); err != nil {
		return err
	}

	ins.addPullInfo(info.Digest.String(), info)
	cfgInfo := imagePullInfo{
		Descriptor: manifest.Config,
	}
	ins.addPullInfo(cfgInfo.Digest.String(), cfgInfo)
	for _, layer := range manifest.Layers {
		layerInfo := imagePullInfo{
			Descriptor: layer,
		}
		ins.addPullInfo(layerInfo.Digest.String(), layerInfo)
	}

	a.SetArtifactMetadata(meta)
	a.SetResponseApproved(ins, "valid OCI image manifest").Annotate(notes)
	return nil
}

// validateImageIndex checks whether the artifact is truly an OCI image blob.
// The blob can be a config file as well and is checked by parsing.
func (ins *OciInspector) validateImageBlob(f ArtifactReader, a ResponseArtifact) error {
	// Blobs must have 'digest' annotated.
	d, ok := a.RequestStringAnnotation(ins.ID(), "digest")
	if !ok {
		return nil
	}
	// Blobs must be referenced from before.
	info, ok := ins.getPullInfo(d)
	if !ok {
		return nil
	}

	// There are not many interesting things about a blob.
	// Only check the digest and see if it matches.
	if err := checkArtifactDigest(info.Digest, f, a); err != nil {
		return err
	}

	if info.MediaType == spec.MediaTypeImageConfig {
		if !a.MimetypeIs("application/json") {
			return nil
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if _, err := parseOciImageConfig(f); err != nil {
			return err
		}
	}

	meta, notes := parseOciAnnotations(info.Annotations)
	if meta.Type == "" {
		meta.Type = info.MediaType
	}
	if meta.Name == "" {
		meta.Name = info.image
	}
	if meta.Version == "" {
		meta.Version = info.reference
	}
	if meta.Description == "" {
		meta.Description = fmt.Sprintf("%s:%s image blob", info.image, info.reference)
	}
	a.SetArtifactMetadata(meta)
	a.SetResponseApproved(ins, "valid OCI image blob").Annotate(notes)
	return nil
}

func (ins *OciInspector) extractImageIndexInfo(index *spec.Index, a ResponseArtifact) (imagePullInfo, ArtifactMetadata, Annotation) {
	info := ins.getOrMakePullInfo(a)
	info.MediaType = index.MediaType
	info.ArtifactType = index.ArtifactType
	for k, v := range index.Annotations {
		info.Annotations[k] = v
	}

	meta, notes := parseOciAnnotations(info.Annotations)
	if meta.Type == "" {
		meta.Type = spec.MediaTypeImageIndex
	}
	if meta.Name == "" {
		meta.Name = info.image
	}
	if meta.Version == "" {
		meta.Version = info.reference
	}
	if meta.Description == "" {
		meta.Description = fmt.Sprintf("%s:%s image index", info.image, info.reference)
	}
	return info, meta, notes
}

func (ins *OciInspector) extractImageManifestInfo(mfest *spec.Manifest, a ResponseArtifact) (imagePullInfo, ArtifactMetadata, Annotation) {
	info := ins.getOrMakePullInfo(a)
	info.MediaType = mfest.MediaType
	info.ArtifactType = mfest.ArtifactType
	for k, v := range mfest.Annotations {
		info.Annotations[k] = v
	}

	meta, notes := parseOciAnnotations(info.Annotations)
	if meta.Type == "" {
		meta.Type = spec.MediaTypeImageManifest
	}
	if meta.Name == "" {
		meta.Name = info.image
	}
	if meta.Version == "" {
		meta.Version = info.reference
	}
	if meta.Description == "" {
		meta.Description = fmt.Sprintf("%s:%s image manifest", info.image, info.reference)
	}
	return info, meta, notes
}

func (ins *OciInspector) getOrMakePullInfo(a ResponseArtifact) imagePullInfo {
	var info *imagePullInfo
	if d, ok := a.RequestStringAnnotation(ins.ID(), "digest"); ok {
		v, ok := ins.getPullInfo(d)
		if ok {
			info = &v
		}
	}
	if info == nil {
		info = &imagePullInfo{}
	}
	if image, ok := a.RequestStringAnnotation(ins.ID(), "image-name"); ok && info.image == "" {
		info.image = image
	}
	if ref, ok := a.RequestStringAnnotation(ins.ID(), "reference"); ok && info.reference == "" {
		info.reference = ref
	}
	if info.Digest == "" {
		info.Digest = digest.NewDigestFromEncoded(digest.SHA256, a.Sha256().String())
	}
	if info.Size == 0 {
		info.Size = a.Size()
	}
	return *info
}
