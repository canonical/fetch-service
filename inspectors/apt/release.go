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

package apt

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"

	apt_cfg "github.com/canonical/fetch-service/inspectors/apt/config"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata/digests"
)

// Distribution Release/InRelease file
// (http://archive.ubuntu.com/ubuntu/dists/jammy/InRelease)
//
// Example content:
//
// -----BEGIN PGP SIGNED MESSAGE-----
// Hash: SHA512
//
// Origin: Ubuntu
// Label: Ubuntu
// Suite: jammy-backports
// Version: 22.04
// Codename: jammy
// Date: Fri, 07 Jul 2023 18:13:42 UTC
// Architectures: amd64 arm64 armhf i386 ppc64el riscv64 s390x
// Components: main restricted universe multiverse
// Description: Ubuntu Jammy Backports
// NotAutomatic: yes
// ButAutomaticUpgrades: yes
// ...
// SHA1:
// ...
//  92846831bdb75c027c763ea21c8b0d86dfefe4a2         43739952 Contents-s390x.gz
//  3569f72ef54ca6b7a374846358ae77424ee3845b          6779186 main/binary-amd64/Packages
//  b5230c4d59d77d91fdcefe76135df12737a1bc2f          1792213 main/binary-amd64/Packages.gz
//  370c66437d49460dbc16be011209c4de9977212d          1394768 main/binary-amd64/Packages.xz
// ...

// releaseEntry holds information about metadata files listed in the Release file.
type releaseEntry struct {
	Name string // file path
	Size uint64 // size of entry
}

// releaseFile holds information about the Release file
type releaseFile struct {
	Sha256 digests.Sha256Digest                  // SHA256 digest of the release file
	Vendor string                                // release file vendor
	Files  map[digests.Sha256Digest]releaseEntry // file entries listed in this release file
}

func NewReleaseFile() releaseFile {
	return releaseFile{
		Files: make(map[digests.Sha256Digest]releaseEntry, 100),
	}
}

// The AptReleaseInspector inspects signed InRelease files.
type AptReleaseInspector struct {
	release     map[string]releaseFile // map repository to release file
	releaseLock sync.Mutex

	config apt_cfg.AptInspectorConfig
}

func NewAptReleaseInspector(cfg apt_cfg.AptInspectorConfig) *AptReleaseInspector {
	return &AptReleaseInspector{
		release: make(map[string]releaseFile),
		config:  cfg,
	}
}

const aptReleaseInspectorID = "apt.release"

func (ins *AptReleaseInspector) ID() string {
	return aptReleaseInspectorID
}

// InspectRequest verifies if the request is valid for the types of
// files handled by this inspector: InRelease, Packages.xz, and Translation.
func (ins *AptReleaseInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	slog := a.Logger()

	if info, err := apt_cfg.NewInReleaseURLInfo(u, &ins.config, slog); err == nil {
		a.SetRequestPending(ins, "valid URL for Release file").Annotate(
			Annotation{
				"cfg-name":   info.CfgName,
				"origin":     info.Origin,
				"repository": info.Repository,
				"suite":      info.Suite,
			},
		)
	} else if info, err := apt_cfg.NewPackagesURLInfo(u, &ins.config, slog); err == nil {
		// check if we already have downloaded InReleases from this repo
		notes := Annotation{
			"cfg-name":     info.CfgName,
			"origin":       info.Origin,
			"repository":   info.Repository,
			"suite":        info.Suite,
			"component":    info.Component,
			"architecture": info.Architecture,
		}
		repo := fmt.Sprintf("%s/dists/%s", info.Repository, info.Suite)
		repo, _ = ins.getRepositoryAlias(repo, info.CfgName, slog)

		if _, ok := ins.getReleaseState(repo); ok {
			a.SetRequestPending(ins, "valid URL for packages file").Annotate(notes)
		} else {
			a.SetRequestRejected(ins, "attempt to download packages file before Release").Annotate(notes)
		}
	} else if info, err := apt_cfg.NewTranslationURLInfo(u, &ins.config, slog); err == nil {
		// check if we already have downloaded InReleases from this repo
		notes := Annotation{
			"cfg-name":   info.CfgName,
			"origin":     info.Origin,
			"repository": info.Repository,
			"suite":      info.Suite,
			"component":  info.Component,
		}
		repo := fmt.Sprintf("%s/dists/%s", info.Repository, info.Suite)
		repo, _ = ins.getRepositoryAlias(repo, info.CfgName, slog)

		if _, ok := ins.getReleaseState(repo); ok {
			a.SetRequestPending(ins, "valid URL for translation file").Annotate(notes)
		} else {
			a.SetRequestRejected(ins, "attempt to download translation file before Release").Annotate(notes)
		}
	} else if info, err := apt_cfg.NewCommandURLInfo(u, &ins.config, slog); err == nil {
		// check if we already have downloaded InReleases from this repo
		notes := Annotation{
			"cfg-name":   info.CfgName,
			"origin":     info.Origin,
			"repository": info.Repository,
			"suite":      info.Suite,
			"component":  info.Component,
		}
		repo := fmt.Sprintf("%s/dists/%s", info.Repository, info.Suite)
		repo, _ = ins.getRepositoryAlias(repo, info.CfgName, slog)

		if _, ok := ins.getReleaseState(repo); ok {
			a.SetRequestPending(ins, "valid URL for commands file").Annotate(notes)
		} else {
			a.SetRequestRejected(ins, "attempt to download commands file before Release").Annotate(notes)
		}
	}

	return nil
}

// InspectArtifact examines InRelease files and validates Packages.xz files
// against InRelease entries.
func (ins *AptReleaseInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if a.MimetypeIs(mimetypes.AptPackages) {
		return ins.validatePackagesFile(f, a)
	}

	if a.MimetypeIs(mimetypes.AptTranslation) {
		return ins.validateTranslationFile(f, a)
	}

	if a.MimetypeIs(mimetypes.AptCommands) {
		return ins.validateCommandsFile(f, a)
	}

	if !a.MimetypeIs("text/plain") {
		return nil // not a Release file
	}

	slog := a.Logger()

	// Check if this is a valid InRelease file

	formatErrors := []string{}
	integrityErrors := []string{}

	cfgName, ok := a.RequestStringAnnotation(ins.ID(), "cfg-name")
	if !ok {
		return nil
	}
	slog.Debugf("check repository config entry '%s'", cfgName)

	// Quick check for clearsigned file
	buf := make([]byte, 34)
	n, err := f.Read(buf)
	if err != nil || n != 34 {
		return nil // not our clearsigned file
	}
	if !bytes.Equal(buf, []byte("-----BEGIN PGP SIGNED MESSAGE-----")) {
		return nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// InRelease files must be signed
	signotes := Annotation{}
	pubkey := ins.config.Repositories[cfgName].PublicKey
	slog.Debugf("apt repository public key: %s", pubkey)
	body, err := checkSignature(f, signotes, pubkey)
	if err != nil {
		slog.Warningf("signature checking error: %s", err)
		integrityErrors = append(integrityErrors, fmt.Sprintf("signature verification failed: %s", err))

		// Update the reader if the file is not clearsigned
		if body == nil {
			body = f
			if _, err := body.Seek(0, io.SeekStart); err != nil {
				return err
			}
		}
	}

	sc := bufio.NewScanner(body)
	sc.Split(bufio.ScanLines)

	sha256Section := false

	fields := map[string]string{}

	n = 0
	for sc.Scan() {
		line := sc.Text()
		if len(line) == 0 || line[0] == ' ' {
			continue
		}

		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if k == "SHA256" {
			sha256Section = true
			break
		}
		v = strings.TrimSpace(v)

		if v != "" {
			fields[k] = v
		}

		if n > 100 {
			return nil // this doesn't look like a Release file
		}
		n++
	}

	if !sha256Section {
		slog.Debug("no SHA256 section found")
		return nil // we don't recognize this file
	}

	// Check if all expected fields are in place
	expectedFields := []string{
		"Origin",
		"Label",
		"Suite",
		"Version",
		"Codename",
		"Date",
		"Architectures",
		"Components",
	}

	for _, k := range expectedFields {
		_, ok := fields[k]
		if !ok {
			slog.Debugf("expected field %q not found", k)
			return nil // we don't recognize this file
		}
	}

	// We now assume this is an InRelease file
	slog.Debug("validate release file")

	release := NewReleaseFile()
	release.Sha256 = a.Sha256()
	release.Vendor = fields["Origin"]

	for sc.Scan() {
		line := sc.Text()
		if line[0] != ' ' {
			break
		}

		// Read list of sha256_section and metadata files
		f := strings.Fields(line)
		if len(f) != 3 {
			formatErrors = append(formatErrors, fmt.Sprintf("ill-formed line: %s", line))
			continue
		}
		filepath := f[2]

		digest, err := digests.NewSha256Digest(f[0])
		if err != nil {
			formatErrors = append(formatErrors, fmt.Sprintf("error parsing digest: %s", line))
			continue
		}

		size, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil {
			formatErrors = append(formatErrors, fmt.Sprintf("error parsing file size: %s", line))
			continue
		}

		entry := releaseEntry{Size: size, Name: filepath}
		release.Files[digest] = entry
	}

	repo := strings.TrimSuffix(a.DownloadURL(), "/InRelease")

	desc := fields["Description"]
	if desc == "" {
		desc = fmt.Sprintf("%s %s", fields["Origin"], fields["Suite"])
	}
	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.AptRelease,
		Name:        "InRelease",
		Version:     fields["Codename"],
		Description: desc,
		Vendor:      fields["Origin"],
		Author:      fields["Origin"],
		AptSuite:    fields["Suite"],
	})

	notes := Annotation{}

	if len(formatErrors) > 0 {
		notes.Add("file format errors", formatErrors)
	}
	if len(integrityErrors) > 0 {
		notes.Add("integrity errors", integrityErrors)
	}

	if len(notes) > 0 {
		a.SetResponseRejected(ins, "error parsing release file").Annotate(notes)
		return nil
	}

	for k, v := range fields {
		notes.Add(k, v)
	}
	for k, v := range signotes {
		notes.Add(k, v)
	}

	a.SetResponseApproved(ins, "release file parsed and signature validated").Annotate(notes)

	repo, _ = ins.getRepositoryAlias(repo, cfgName, slog)
	ins.setReleaseState(repo, release)

	return nil
}

func (ins *AptReleaseInspector) getRepositoryAlias(repo, cfgName string, slog logger.Logger) (string, bool) {
	repos, ok := ins.config.Repositories[cfgName]
	if !ok {
		slog.Debugf("%s: repository configuration not found", cfgName)
		return repo, false
	}

	baseURL := repos.BaseURLAlias
	if baseURL == "" {
		slog.Debugf("repository not aliased: %s", repo)
		return repo, true
	}

	u, err := url.Parse(repo)
	if err != nil {
		slog.Debugf("error parsing repository URL: %s", err)
		return repo, true
	}

	alias, err := url.JoinPath(baseURL, u.Path)
	if err != nil {
		slog.Debugf("error creating url alias: %s", err)
		return repo, true
	}

	slog.Debugf("repository url alias: %s to %s", repo, alias)
	return alias, true
}

func (ins *AptReleaseInspector) setReleaseState(repo string, release releaseFile) {
	ins.releaseLock.Lock()
	defer ins.releaseLock.Unlock()

	ins.release[repo] = release
}

func (ins *AptReleaseInspector) getReleaseState(repo string) (releaseFile, bool) {
	ins.releaseLock.Lock()
	defer ins.releaseLock.Unlock()

	rel, ok := ins.release[repo]
	return rel, ok
}

func (ins *AptReleaseInspector) validatePackagesFile(f ArtifactReader, a ResponseArtifact) error {
	slog := a.Logger()
	slog.Debug("validate package file")

	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	slog.Debugf("packages file path: %s", u.Path)
	info, err := apt_cfg.NewPackagesURLInfo(u, &ins.config, slog)
	if err != nil {
		a.SetResponseRejected(ins, "invalid path for packages file")
		return nil
	}

	if info.Digest != "" {
		sha256, err := digests.NewSha256Digest(info.Digest)
		if err != nil {
			a.SetResponseRejected(ins, "invalid SHA256 digest")
			return nil
		}
		slog.Debugf("by-hash SHA256 digest: %s", info.Digest)

		if sha256 != a.Sha256() {
			a.SetResponseRejected(ins, "SHA256 digest mismatch").Annotate(
				Annotation{"expected-sha256": sha256.String()},
			)
			return nil
		}
	}

	cfgName, ok := a.RequestStringAnnotation(ins.ID(), "cfg-name")
	if !ok {
		a.SetResponseRejected(ins, "Packages file downloaded from unknown repository")
		return nil
	}
	repo := fmt.Sprintf("%s/dists/%s", info.Repository, info.Suite)
	repo, ok = ins.getRepositoryAlias(repo, cfgName, slog)
	if !ok {
		a.SetResponseRejected(ins, "Unknown repository configuration name").Annotate(
			Annotation{"cfg-name": cfgName},
		)
		return nil
	}

	rel, ok := ins.getReleaseState(repo)
	if !ok {
		a.SetResponseRejected(ins, "Repository Release data not found")
		return nil
	}

	entry, ok := rel.Files[a.Sha256()]
	if !ok {
		a.SetResponseRejected(ins, "Packages file not listed in Release file")
		return nil
	}
	slog.Debugf("release entry: %+v", entry)

	a.SetResponseUnknown(ins, "Packages file listed in Release").Annotate(
		Annotation{
			"file-path":    entry.Name,
			"release-file": rel.Sha256.String(),
			"vendor":       rel.Vendor,
		},
	)

	return nil
}

// validateTranslationFile examines InRelease files and validates Translation-<lang>
// files against InRelease entries.
// https://wiki.debian.org/DebianRepository/Format#A.22Translation.22_indices
func (ins *AptReleaseInspector) validateTranslationFile(f ArtifactReader, a ResponseArtifact) error {
	slog := a.Logger()
	slog.Debug("validate translation file")

	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	slog.Debugf("translation file path: %s", u.Path)
	info, err := apt_cfg.NewTranslationURLInfo(u, &ins.config, slog)
	if err != nil {
		a.SetResponseRejected(ins, "invalid path for translation file")
		return nil
	}

	cfgName, ok := a.RequestStringAnnotation(ins.ID(), "cfg-name")
	if !ok {
		a.SetResponseRejected(ins, "Translation file downloaded from unknown repository")
		return nil
	}
	repo := fmt.Sprintf("%s/dists/%s", info.Repository, info.Suite)
	repo, ok = ins.getRepositoryAlias(repo, cfgName, slog)
	if !ok {
		a.SetResponseRejected(ins, "Unknown repository configuration name").Annotate(
			Annotation{"cfg-name": cfgName},
		)
		return nil
	}

	rel, ok := ins.getReleaseState(repo)
	if !ok {
		a.SetResponseRejected(ins, "Repository Release data not found")
		return nil
	}

	entry, ok := rel.Files[a.Sha256()]
	if !ok {
		a.SetResponseRejected(ins, "Translation file not listed in Release file")
		return nil
	}
	slog.Debugf("release entry: %+v", entry)

	if int64(entry.Size) != a.Size() {
		a.SetResponseRejected(ins, "Translation file size mismatch").Annotate(
			Annotation{
				"expected-size": entry.Size,
			},
		)
		return nil
	}

	a.SetResponseUnknown(ins, "Translation file listed in Release").Annotate(
		Annotation{
			"file-path":    entry.Name,
			"release-file": rel.Sha256.String(),
			"vendor":       rel.Vendor,
		},
	)

	return nil
}

// validateCommandsFile examines InRelease files and validates Commands-<arch>
// files against InRelease entries.
func (ins *AptReleaseInspector) validateCommandsFile(f ArtifactReader, a ResponseArtifact) error {
	slog := a.Logger()
	slog.Debug("validate commands file")

	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	slog.Debugf("commands file path: %s", u.Path)
	info, err := apt_cfg.NewCommandURLInfo(u, &ins.config, slog)
	if err != nil {
		a.SetResponseRejected(ins, "invalid path for commands file")
		return nil
	}

	cfgName, ok := a.RequestStringAnnotation(ins.ID(), "cfg-name")
	if !ok {
		a.SetResponseRejected(ins, "Commands file downloaded from unknown repository")
		return nil
	}
	repo := fmt.Sprintf("%s/dists/%s", info.Repository, info.Suite)
	repo, ok = ins.getRepositoryAlias(repo, cfgName, slog)
	if !ok {
		a.SetResponseRejected(ins, "Unknown repository configuration name").Annotate(
			Annotation{"cfg-name": cfgName},
		)
		return nil
	}

	rel, ok := ins.getReleaseState(repo)
	if !ok {
		a.SetResponseRejected(ins, "Repository Release data not found")
		return nil
	}

	entry, ok := rel.Files[a.Sha256()]
	if !ok {
		a.SetResponseRejected(ins, "Commands file not listed in Release file")
		return nil
	}
	slog.Debugf("release entry: %+v", entry)

	if int64(entry.Size) != a.Size() {
		a.SetResponseRejected(ins, "Commands file size mismatch").Annotate(
			Annotation{
				"expected-size": entry.Size,
			},
		)
		return nil
	}

	a.SetResponseUnknown(ins, "Commands file listed in Release").Annotate(
		Annotation{
			"file-path":    entry.Name,
			"release-file": rel.Sha256.String(),
			"vendor":       rel.Vendor,
		},
	)

	return nil
}
