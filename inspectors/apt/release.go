// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024 Canonical Ltd.
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

// The apt Release file inspector inspects signed InRelease files.
type AptReleaseInspector struct {
	release map[string]releaseFile // map repository to release file

	releaseLock sync.Mutex
}

func NewAptReleaseInspector() *AptReleaseInspector {
	return &AptReleaseInspector{
		release: make(map[string]releaseFile),
	}
}

func (ins *AptReleaseInspector) ID() string {
	return "apt.release"
}

// AptReleaseInspector verifies if the request is valid for the types of
// files handled by this inspector: InRelease, Packages.xz, and Translation.
func (ins *AptReleaseInspector) InspectRequest(a RequestArtefact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if info, err := newInReleaseUrlInfo(u); err == nil {
		a.SetRequestPending(ins, "valid URL for Release file").Annotate(
			Annotation{
				"origin":     info.origin,
				"repository": info.repository,
				"dist":       info.dist,
			},
		)
		return nil
	}

	if info, err := newPackagesUrlInfo(u); err == nil {
		// check if we already have downloaded InReleases from this repo
		notes := Annotation{
			"origin":       info.origin,
			"repository":   info.repository,
			"dist":         info.dist,
			"component":    info.component,
			"architecture": info.architecture,
		}
		repo := fmt.Sprintf("%s/dists/%s", info.repository, info.dist)

		_, ok := ins.release[repo]
		if ok {
			a.SetRequestPending(ins, "valid URL for packages file").Annotate(notes)
		} else {
			a.SetRequestRejected(ins, "attempt to download packages file before Release").Annotate(notes)
		}
		return nil
	}

	if info, err := newTranslationUrlInfo(u); err == nil {
		// check if we already have downloaded InReleases from this repo
		notes := Annotation{
			"origin":     info.origin,
			"repository": info.repository,
			"dist":       info.dist,
			"component":  info.component,
		}
		repo := fmt.Sprintf("%s/dists/%s", info.repository, info.dist)

		_, ok := ins.release[repo]
		if ok {
			a.SetRequestPending(ins, "valid URL for translation file").Annotate(notes)
		} else {
			a.SetRequestRejected(ins, "attempt to download translation file before Release").Annotate(notes)
		}
	}

	return nil
}

// InspectArtefact examines InRelease files and validates Packages.xz files
// against InRelease entries.
func (ins *AptReleaseInspector) InspectArtefact(f ArtefactFile, a ResponseArtefact) error {
	if a.MimetypeIs(mimetypes.AptPackages) {
		return ins.validatePackagesFile(f, a)
	}

	if a.MimetypeIs(mimetypes.AptTranslation) {
		return ins.validateTranslationFile(f, a)
	}

	if !a.MimetypeIs("text/plain") {
		return nil // not a Release file
	}

	// Check if this is a valid InRelease file

	format_errors := []string{}
	integrity_errors := []string{}

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
	body, err := checkSignature(f, signotes)
	if err != nil {
		logger.Warningf("signature checking error: %s", err)
		integrity_errors = append(integrity_errors, fmt.Sprintf("signature verification failed: %s", err))

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

	sha256_section := false

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
			sha256_section = true
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

	if !sha256_section {
		logger.Debug("no SHA256 section found")
		return nil // we don't recognize this file
	}

	// Check if all expected fields are in place
	expected_fields := []string{
		"Origin",
		"Label",
		"Suite",
		"Version",
		"Codename",
		"Date",
		"Architectures",
		"Components",
		"Description",
	}

	for _, k := range expected_fields {
		_, ok := fields[k]
		if !ok {
			logger.Debugf("expected field %q not found", k)
			return nil // we don't recognize this file
		}
	}

	// We now assume this is an InRelease file
	logger.Debug("validate release file")

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
			format_errors = append(format_errors, fmt.Sprintf("ill-formed line: %s", line))
			continue
		}
		filepath := f[2]

		digest, err := digests.NewSha256Digest(f[0])
		if err != nil {
			format_errors = append(format_errors, fmt.Sprintf("error parsing digest: %s", line))
			continue
		}

		size, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil {
			format_errors = append(format_errors, fmt.Sprintf("error parsing file size: %s", line))
			continue
		}

		entry := releaseEntry{Size: size, Name: filepath}
		release.Files[digest] = entry
	}

	repo := strings.TrimSuffix(a.DownloadURL(), "/InRelease")

	a.SetArtefactMetadata(ArtefactMetadata{
		Type:        mimetypes.AptRelease,
		Name:        "InRelease",
		Version:     fields["Codename"],
		Description: fields["Description"],
		Vendor:      fields["Origin"],
		Author:      fields["Origin"],
	})

	notes := Annotation{}

	if len(format_errors) > 0 {
		notes.Add("file format errors", format_errors)
	}
	if len(integrity_errors) > 0 {
		notes.Add("integrity errors", integrity_errors)
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

	ins.releaseLock.Lock()
	defer ins.releaseLock.Unlock()

	ins.release[repo] = release

	return nil
}

func (ins *AptReleaseInspector) validatePackagesFile(f ArtefactFile, a ResponseArtefact) error {
	logger.Debug("validate package file")

	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	logger.Debugf("packages file path: %s", u.Path)
	info, err := newPackagesUrlInfo(u)
	if err != nil {
		a.SetResponseRejected(ins, "invalid path for packages file")
		return nil
	}

	sha256, err := digests.NewSha256Digest(info.digest)
	if err != nil {
		a.SetResponseRejected(ins, "invalid SHA256 digest: %s", err)
		return nil
	}
	logger.Debugf("by-hash SHA256 digest: %s", info.digest)

	repo := fmt.Sprintf("%s/dists/%s", info.repository, info.dist)
	rel, ok := ins.release[repo]
	if !ok {
		a.SetResponseRejected(ins, "Release data not found for this repository")
		return nil
	}

	entry, ok := rel.Files[sha256]
	if !ok {
		a.SetResponseRejected(ins, "Packages file not listed in Release file")
		return nil
	}
	logger.Debugf("release entry: %+v", entry)

	if sha256 != a.Sha256() {
		a.SetResponseRejected(ins, "SHA256 digest mismatch").Annotate(
			Annotation{
				"expected-sha256": sha256.String(),
			},
		)
		return nil
	}

	a.SetResponseApproved(ins, "Packages file listed in Release").Annotate(
		Annotation{
			"file-path":    entry.Name,
			"release-file": ins.release[repo].Sha256.String(),
			"vendor":       rel.Vendor,
		},
	)

	return nil
}

// validateTranslationFile examines InRelease files and validates Translation-<lang>
// files against InRelease entries.
// https://wiki.debian.org/DebianRepository/Format#A.22Translation.22_indices
func (ins *AptReleaseInspector) validateTranslationFile(f ArtefactFile, a ResponseArtefact) error {
	logger.Debug("validate translation file")

	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	logger.Debugf("translation file path: %s", u.Path)
	info, err := newTranslationUrlInfo(u)
	if err != nil {
		a.SetResponseRejected(ins, "invalid path for translation file")
		return nil
	}

	repo := fmt.Sprintf("%s/dists/%s", info.repository, info.dist)
	rel, ok := ins.release[repo]
	if !ok {
		a.SetResponseRejected(ins, "Release data not found for this repository")
		return nil
	}

	entry, ok := rel.Files[a.Sha256()]
	if !ok {
		a.SetResponseRejected(ins, "Translation file not listed in Release file")
		return nil
	}
	logger.Debugf("release entry: %+v", entry)

	if int64(entry.Size) != a.Size() {
		a.SetResponseRejected(ins, "Translation file size mismatch").Annotate(
			Annotation{
				"expected-size": entry.Size,
			},
		)
		return nil
	}

	a.SetResponseApproved(ins, "Translation file listed in Release").Annotate(
		Annotation{
			"file-path":    entry.Name,
			"release-file": ins.release[repo].Sha256.String(),
			"vendor":       rel.Vendor,
		},
	)

	return nil
}
