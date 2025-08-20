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

package chisel

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"regexp"

	"gopkg.in/yaml.v3"

	apt_cfg "github.com/canonical/fetch-service/inspectors/apt/config"
	"github.com/canonical/fetch-service/inspectors/chisel/config"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/utils"
)

// The Chisel release inspector verifies the chisel-releases repository's [1]
// tarball download request and response artifact.
//
// Chisel [2] downloads the tarball via a GET request to
// https://codeload.github.com. The inspector monitors this request and
// currently only examines the gzip compressed tarball and checks if it contains
// appropriate files.
//
// [1] https://github.com/canonical/chisel-releases
// [2] https://github.com/canonical/chisel
type ChiselReleaseInspector struct {
	cfg    *config.ChiselInspectorConfig
	aptCfg *apt_cfg.AptInspectorConfig
}

func NewChiselReleaseInspector(cfg config.ChiselInspectorConfig, aptCfg apt_cfg.AptInspectorConfig) *ChiselReleaseInspector {
	return &ChiselReleaseInspector{
		cfg:    &cfg,
		aptCfg: &aptCfg,
	}
}

func (ins *ChiselReleaseInspector) ID() string {
	return "chisel.release"
}

// InspectRequest verifies whether this is a valid chisel-release fetch request.
//
// The URL must match with any of the URL patterns defined in
// [config.ChiselInspectorConfig].
func (ins *ChiselReleaseInspector) InspectRequest(a RequestArtifact) error {
	url := a.DownloadURL()
	for _, pattern := range ins.cfg.Urls {
		if pattern.G.Match(url) {
			a.SetRequestPending(ins, "request matches valid URL")
			return nil
		}
	}

	return nil
}

// InspectArtifact sets an artifact to approved if:
//   - it is a gzip compressed tarball.
//   - it contains a valid "chisel.yaml" file in the top-level
//     "chisel-releases-<release>" directory.
//
// It only rejects an artifact if there is chisel.yaml file inside, but the file
// contents are not valid.
//
// In all other cases, it returns nil, which indicates that it does not
// recognize the artifact.
func (ins *ChiselReleaseInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/gzip") {
		return nil // We do not recognize this artifact.
	}

	zf, err := gzip.NewReader(f)
	if err != nil {
		return nil // We do not recognize this artifact.
	}
	defer zf.Close()

	// Parse the tarball.
	tr := tar.NewReader(zf)
	release, data, err := inspectTarball(tr)
	if err != nil {
		if err == errUnrecognized {
			return nil // We do not recognize this artifact.
		}
		a.SetResponseRejected(ins, fmt.Sprintf("invalid tarball: %s", err))
		return err
	}

	if err := validPubKeys(ins.aptCfg, data); err != nil {
		a.SetResponseRejected(ins, fmt.Sprintf("invalid public-keys: %s", err))
		return err
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.ChiselRelease,
		Name:        "chisel-release",
		Version:     data.Format,
		Description: fmt.Sprintf("Chisel release file for %s", release),
		Vendor:      "Canonical",
	})
	a.SetResponseApproved(ins, "artifact successfully parsed")
	return nil
}

var errUnrecognized = errors.New("unrecognized artifact")

// inspectTarball inspects the artifact tarball and checks that chiselPath and
// slicesDir are present there.
// It parses and returns the chisel.yaml file. It returns an error if those
// files are not present or the parsed chisel.yaml is not valid.
func inspectTarball(r *tar.Reader) (string, *chiselYaml, error) {
	chiselYamlPath := regexp.MustCompile("^chisel-releases-(.*)/chisel.yaml$")
	for {
		h, err := r.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", nil, errUnrecognized
		}
		match := chiselYamlPath.FindStringSubmatch(h.Name)
		if len(match) == 2 {
			data, err := parseChiselYaml(r)
			if err != nil {
				return "", nil, err
			}
			return match[1], data, err
		}
	}
	return "", nil, errUnrecognized
}

// Only a subset of chisel.yaml fields are parsed here.
type chiselYaml struct {
	Format     string                   `yaml:"format"`
	Archives   map[string]chiselArchive `yaml:"archives"`
	PublicKeys map[string]chiselPubKey  `yaml:"public-keys"`
}

type chiselArchive struct {
	Components []string `yaml:"components"`
	Suites     []string `yaml:"suites"`
	PublicKeys []string `yaml:"public-keys"`
}

type chiselPubKey struct {
	ID    string `yaml:"id"`
	Armor string `yaml:"armor"`
}

// parseChiselYAML parses the chisel.yaml file and check if it is valid.
// The file is deemed valid if:
//   - it has a non-empty "format"
//   - it has at least one "archive".
//   - it has at least one "public-key".
//   - each archive has at least one "component", one "suite" and one "public-key".
func parseChiselYaml(r io.Reader) (data *chiselYaml, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("invalid chisel.yaml: %w", err)
		}
	}()

	data = &chiselYaml{}
	dec := yaml.NewDecoder(r)
	if err = dec.Decode(data); err != nil {
		return nil, err
	}
	if data.Format == "" || len(data.Archives) == 0 || len(data.PublicKeys) == 0 {
		return nil, fmt.Errorf("missing fields")
	}
	for name, archive := range data.Archives {
		if len(archive.Components) == 0 || len(archive.Suites) == 0 ||
			len(archive.PublicKeys) == 0 {
			return nil, fmt.Errorf("archive %q has missing fields", name)
		}
		for _, key := range archive.PublicKeys {
			if _, ok := data.PublicKeys[key]; !ok {
				return nil, fmt.Errorf("archive %q pubkey %q undefined", name, key)
			}
		}
	}
	for name, key := range data.PublicKeys {
		if key.ID == "" || key.Armor == "" {
			return nil, fmt.Errorf("pubkey %q has missing fields", name)
		}
	}
	return data, nil
}

// validPubKeys returns true if one of the public keys defined in chisel.yaml
// matches any defined in the apt repository config.
func validPubKeys(aptCfg *apt_cfg.AptInspectorConfig, data *chiselYaml) error {
	for keyName, keyData := range data.PublicKeys {
		pubKeys, err := utils.DecodePubKey([]byte(keyData.Armor), false)
		if err != nil {
			return fmt.Errorf("cannot parse chisel.yaml public key %s: %w", keyName, err)
		}
		pubKey := pubKeys[0]
		for repoName, repo := range aptCfg.Repositories {
			repoKeys, err := utils.DecodePubKey([]byte(repo.PublicKey), true)
			if err != nil {
				return fmt.Errorf("cannot parse APT repository %s public key: %w", repoName, err)
			}
			for _, repoKey := range repoKeys {
				if repoKey.KeyId == pubKey.KeyId {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("invalid chisel.yaml: no public key is present in APT configuration")
}
