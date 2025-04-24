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
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/opencontainers/go-digest"
	spec "github.com/opencontainers/image-spec/specs-go/v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/oci/config"
	"github.com/canonical/fetch-service/logger"
)

// registryIsAllowed checks whether registry URL ru is allowed per the
// configuration. If allowed, it returns the configured registry name and a
// boolean set to true. Otherwise, it returns an empty string and false.
func registryIsAllowed(ru string, cfg *config.OciInspectorConfig) (string, bool) {
	logger.Debugf("oci config: check if registry '%s' is allowed", ru)
	for name, registry := range cfg.Registries {
		logger.Debugf("oci config: check registry entry '%s'", name)
		if registry.Url.G.Match(ru) {
			logger.Debugf("oci config: found registry '%s'", registry)
			return name, true
		}
	}
	return "", false
}

func OciImageIndexDetector(raw []byte, limit uint32) bool {
	r := bytes.NewReader(raw)
	_, err := parseOciImageIndex(r)
	return err == nil
}

func parseOciImageIndex(r io.Reader) (*spec.Index, error) {
	index := &spec.Index{}
	d := json.NewDecoder(r)
	if err := d.Decode(index); err != nil {
		return nil, fmt.Errorf("cannot parse OCI image index: %w", err)
	}
	if index.SchemaVersion != 2 {
		return nil, fmt.Errorf("invalid OCI image index 'schemaVersion': %d", index.SchemaVersion)
	}
	if index.MediaType != spec.MediaTypeImageIndex {
		return nil, fmt.Errorf("invalid OCI image index 'mediaType': %s", index.MediaType)
	}
	for _, mf := range index.Manifests {
		if mf.MediaType != spec.MediaTypeImageIndex && mf.MediaType != spec.MediaTypeImageManifest {
			return nil, fmt.Errorf("invalid OCI image manifest 'mediaType': %s", mf.MediaType)
		}
		if mf.Digest == "" {
			return nil, fmt.Errorf("invalid OCI image manifest 'digest': <empty>")
		}
	}
	return index, nil
}

func OciImageManifestDetector(raw []byte, limit uint32) bool {
	r := bytes.NewReader(raw)
	_, err := parseOciImageManifest(r)
	return err == nil
}

func parseOciImageManifest(r io.Reader) (*spec.Manifest, error) {
	mf := &spec.Manifest{}
	d := json.NewDecoder(r)
	if err := d.Decode(mf); err != nil {
		return nil, fmt.Errorf("cannot parse OCI image manifest: %w", err)
	}
	if mf.SchemaVersion != 2 {
		return nil, fmt.Errorf("invalid OCI image manifest 'schemaVersion': %d", mf.SchemaVersion)
	}
	if mf.MediaType != spec.MediaTypeImageManifest {
		return nil, fmt.Errorf("invalid OCI image manifest 'mediaType': %s", mf.MediaType)
	}
	if mf.Config.MediaType != spec.MediaTypeImageConfig {
		return nil, fmt.Errorf("invalid OCI image manifest config 'mediaType': %s", mf.Config.MediaType)
	}
	if mf.Config.Digest == "" {
		return nil, fmt.Errorf("invalid OCI image manifest config 'digest': %s", mf.Config.Digest)
	}
	if len(mf.Layers) == 0 {
		return nil, fmt.Errorf("invalid OCI image manifest 'layers': <empty>")
	}
	return mf, nil
}

func parseOciImageConfig(r io.Reader) (*spec.Image, error) {
	cfg := &spec.Image{}
	d := json.NewDecoder(r)
	if err := d.Decode(cfg); err != nil {
		return nil, fmt.Errorf("cannot parse OCI image config: %w", err)
	}
	if cfg.Architecture == "" {
		return nil, fmt.Errorf("invalid OCI image config 'architecture': <empty>")
	}
	if cfg.OS == "" {
		return nil, fmt.Errorf("invalid OCI image config 'os': <empty>")
	}
	if cfg.RootFS.Type != "layers" {
		return nil, fmt.Errorf("invalid OCI image config 'rootfs.type': %s", cfg.RootFS.Type)
	}
	if len(cfg.RootFS.DiffIDs) == 0 {
		return nil, fmt.Errorf("invalid OCI image config 'rootfs.layers': <empty>")
	}
	return cfg, nil
}

func parseOciAnnotations(m map[string]string) (ArtifactMetadata, Annotation) {
	md := ArtifactMetadata{}
	nt := Annotation{}
	for k, v := range m {
		switch k {
		case spec.AnnotationTitle:
			md.Name = v
		case spec.AnnotationVersion:
			md.Version = v
		case spec.AnnotationVendor:
			md.Vendor = v
		case spec.AnnotationDescription:
			md.Description = v
		case spec.AnnotationAuthors:
			md.Author = v
		case spec.AnnotationLicenses:
			md.License = v
		case spec.AnnotationSource:
			md.SourcePackage = v
		case spec.AnnotationRevision:
			md.StoreRevision = v
		}
		nt[k] = v
	}
	return md, nt
}

// checkArtifactDigest checks if the artifact digest matches the expected
// digest. It returns an error if it doesn't match.
func checkArtifactDigest(expected digest.Digest, f ArtifactReader, a ResponseArtifact) error {
	alg := expected.Algorithm()
	if alg == digest.SHA256 {
		if expected.Encoded() != a.Sha256().String() {
			return fmt.Errorf("invalid digest sha256:%s, expected %s", a.Sha256(), expected)
		}
		return nil
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	d, err := alg.FromReader(f)
	if err != nil {
		return err
	}
	if d != expected {
		return fmt.Errorf("invalid digest %s, expected %s", d, expected)
	}
	return nil
}
