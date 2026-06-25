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
	"compress/gzip"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/xi2/xz"

	apt_cfg "github.com/canonical/fetch-service/inspectors/apt/config"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/utils"
)

// Component Packages.xz file
// (http://archive.ubuntu.com/ubuntu/dists/jammy/universe/binary-amd64/Packages.xz)
//
// Example content:
//
// Package: 0ad
// Architecture: amd64
// Version: 0.0.25b-2
// Priority: optional
// Section: universe/games
// Origin: Ubuntu
// Maintainer: Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>
// Original-Maintainer: Debian Games Team <pkg-games-devel@lists.alioth.debian.org>
// Bugs: https://bugs.launchpad.net/ubuntu/+filebug
// Installed-Size: 21883
// Pre-Depends: dpkg (>= 1.15.6~)
// Depends: 0ad-data (>= 0.0.25b), 0ad-data (<= 0.0.25b-2), 0ad-data-common (>= 0.0.25b), ...
// Filename: pool/universe/0/0ad/0ad_0.0.25b-2_amd64.deb
// Size: 7446276
// MD5sum: 2da5687f8a6530fffd3e917366bc4082
// SHA1: 5ccae1d407cf606f4e944dae9ab02cffb990ebe0
// SHA256: 180288603db5d559d2b7420e84a1b54491c690157bc5a491ae17544a87967424
// SHA512: 7a6ae66d414ecf580f79ed8fdeddf8bd4b3634ba5f2766df8f239529f2abbb22d1c5b57da83e278...
// Homepage: https://play0ad.com/
// Description: Real-time strategy game of ancient warfare
// Description-md5: d943033bedada21853d2ae54a2578a7b
//
// Package: 0ad-data
// Architecture: all
// ...

func AptPackagesDetector(raw []byte, limit uint32) bool {
	r, err := compressedReader(bytes.NewReader(raw))
	if err != nil {
		return false
	}

	buf := make([]byte, 4096)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}

	buf = buf[:n]

	sc := bufio.NewScanner(bytes.NewReader(buf))
	sc.Split(bufio.ScanLines)

	fields := map[string]bool{}

	for sc.Scan() {
		line := sc.Text()
		if len(line) == 0 {
			break
		}

		k, _, ok := strings.Cut(line, ":")
		if !ok {
			return false
		}

		if k == "Size" {
			break // We have enough data to work on
		}

		fields[k] = true
	}

	// Check if we have at least these fields
	if fields["Package"] && fields["Architecture"] && fields["Version"] && fields["Built-Using"] {
		return true
	}

	// If not, having all these is also good
	expectedFields := []string{
		"Package",
		"Architecture",
		"Version",
		"Priority",
		"Section",
		"Maintainer",
	}

	for _, k := range expectedFields {
		_, ok := fields[k]
		if !ok {
			logger.Debugf("apt packages detector: expected field %q not found", k)
			return false // we don't recognize this file
		}
	}

	return true
}

func compressedReader(r io.ReadSeeker) (io.Reader, error) {
	xzr, err := xz.NewReader(r, 0)
	if err == nil {
		return xzr, nil
	}

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	gzr, err := gzip.NewReader(r)
	if err == nil {
		return gzr, nil
	}

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	// Fall back to treating it as an uncompressed (plain text) file
	return r, nil
}

// aptPackagesEntry stores selected fields from each package listed in
// each downloaded Packages.* file.
type aptPackagesEntry struct {
	Pkg          string // package name
	Architecture string // package architecture
	Version      string // package version
	Size         int64  // package size in bytes
}

// aptPackages holds information about the Packages.xz file.
type aptPackages struct {
	sha256       digests.Sha256Digest // packages file digest
	origin       string               // URL origin of the archive
	suite        string               // name of the suite (<series>-<pocket>)
	component    string               // name of the component
	architecture string

	entries     map[digests.Sha256Digest]aptPackagesEntry
	entriesLock sync.Mutex
}

func newAptPackages(sha256 digests.Sha256Digest, origin, suite, component, architecture string) *aptPackages {
	return &aptPackages{
		sha256:       sha256,
		origin:       origin,
		suite:        suite,
		component:    component,
		architecture: architecture,

		entries: make(map[digests.Sha256Digest]aptPackagesEntry),
	}
}

// getPackagesEntry retrieves package information from the inspector state.
func (pkg *aptPackages) getPackagesEntry(digest digests.Sha256Digest) (aptPackagesEntry, bool) {
	pkg.entriesLock.Lock()
	defer pkg.entriesLock.Unlock()

	if e, ok := pkg.entries[digest]; ok {
		return e, true
	}
	return aptPackagesEntry{}, false
}

// AptPackagesInspector contains inspector-specific contextual data for stateful
// analysis within a fetch session.
type AptPackagesInspector struct {
	packages     map[string]map[string]*aptPackages // maps origin to Packages file data
	packagesLock sync.Mutex

	validated     map[digests.Sha256Digest]struct{} // set of validated Packages
	validatedLock sync.Mutex

	config apt_cfg.AptInspectorConfig
}

func NewAptPackagesInspector(cfg apt_cfg.AptInspectorConfig) *AptPackagesInspector {
	return &AptPackagesInspector{
		packages:  make(map[string]map[string]*aptPackages),
		validated: make(map[digests.Sha256Digest]struct{}),
		config:    cfg,
	}
}

func (ins *AptPackagesInspector) ID() string {
	return "apt.packages"
}

func (ins *AptPackagesInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	sl := a.Logger()

	if info, err := apt_cfg.NewPackagesURLInfo(u, &ins.config, sl); err == nil {
		a.SetRequestPending(ins, "valid URL for Packages file").Annotate(
			Annotation{
				"cfg-name":     info.CfgName,
				"repository":   info.Repository,
				"suite":        info.Suite,
				"component":    info.Component,
				"architecture": info.Architecture,
			},
		)
	} else if info, err := apt_cfg.NewDebPackageURLInfo(u, &ins.config, sl); err == nil {
		a.SetRequestPending(ins, "valid URL for deb package").Annotate(
			Annotation{
				"cfg-name":     info.CfgName,
				"repository":   info.Repository,
				"component":    info.Component,
				"name":         info.Name,
				"version":      info.Version,
				"architecture": info.Architecture,
			},
		)
	}

	return nil
}

func (ins *AptPackagesInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if a.MimetypeIs(mimetypes.DebianBinaryPackage) {
		return ins.validateDebianPackage(f, a)
	}

	if !a.MimetypeIs(mimetypes.AptPackages) {
		return nil
	}

	sl := a.Logger()

	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	suite, ok := a.RequestStringAnnotation(ins.ID(), "suite")
	if !ok {
		a.SetResponseUnknown(ins, "suite not specified in request URL", NoMetadata)
		return nil
	}
	component, ok := a.RequestStringAnnotation(ins.ID(), "component")
	if !ok {
		a.SetResponseUnknown(ins, "component not specified in request URL", NoMetadata)
		return nil
	}
	architecture, ok := a.RequestStringAnnotation(ins.ID(), "architecture")
	if !ok {
		a.SetResponseUnknown(ins, "architecture not specified in request URL", NoMetadata)
		return nil
	}

	md := ArtifactMetadata{
		Type:         mimetypes.AptPackages,
		Name:         "Packages",
		Description:  fmt.Sprintf("%s %s Packages file", suite, component),
		Architecture: architecture,
		AptSuite:     suite,
	}

	cfgName, ok := a.RequestStringAnnotation(ins.ID(), "cfg-name")
	if !ok {
		a.SetResponseRejected(ins, "Packages file downloaded from unknown repository", md)
		return nil
	}

	origin, ok := ins.getOriginAlias(utils.NormalizedOrigin(u), cfgName, sl)
	if !ok {
		a.SetResponseRejected(ins, "Unknown repository configuration name", md).Annotate(
			Annotation{"cfg-name": cfgName},
		)
		return nil
	}

	pkg := newAptPackages(a.Sha256(), origin, suite, component, architecture)
	ins.addPackages(origin, u.Path, pkg, a.Logger())

	// the file should be also annotated by the release inspector
	vendor, ok := a.ResponseStringAnnotation(aptReleaseInspectorID, "vendor")
	if ok {
		md.Author = vendor
		md.Vendor = vendor
	}

	// Add packages list to inspector state
	r, err := compressedReader(f)
	if err != nil {
		return err
	}

	var num int
	entries := map[digests.Sha256Digest]aptPackagesEntry{}
	num, err = parsePackages(r, entries, a.Logger())
	if err != nil {
		a.SetResponseRejected(ins, "error parsing packages file", md).Annotate(
			Annotation{
				"error-msg":     err.Error(),
				"package-count": num,
			},
		)
		return nil
	}

	notes := Annotation{"package-count": num}

	releaseDigest, ok := a.ResponseStringAnnotation(aptReleaseInspectorID, "release-file")
	if ok {
		notes["release-file"] = releaseDigest
		ins.validatePackages(a.Sha256())
		a.SetResponseApproved(ins, "packages file successfully parsed", md).Annotate(notes)
	} else {
		a.SetResponseRejected(ins, "packages file not verified against release file", md).Annotate(notes)
	}

	pkg.entriesLock.Lock()
	defer pkg.entriesLock.Unlock()

	pkg.entries = entries

	return nil
}

func (ins *AptPackagesInspector) validatePackages(sha256 digests.Sha256Digest) {
	ins.validatedLock.Lock()
	defer ins.validatedLock.Unlock()
	ins.validated[sha256] = struct{}{}
}

func (ins *AptPackagesInspector) isValidPackages(sha256 digests.Sha256Digest) bool {
	ins.validatedLock.Lock()
	defer ins.validatedLock.Unlock()
	_, ok := ins.validated[sha256]
	return ok
}

func parsePackages(r io.Reader, entries map[digests.Sha256Digest]aptPackagesEntry, sl logger.Logger) (int, error) {
	sc := bufio.NewScanner(r)
	sc.Split(bufio.ScanLines)

	// some lines can be really long (e.g. librust-winapi-dev Provides:)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	var e aptPackagesEntry

	num := 0

	for sc.Scan() {
		line := sc.Text()

		if line == "" {
			e = aptPackagesEntry{}
			continue
		}

		k, v, ok := strings.Cut(line, ":")
		if !ok {
			// It should be safe to ignore this field; the ones we're interested
			// should always be single-line according to
			// https://www.debian.org/doc/debian-policy/ch-controlfields.html#syntax-of-control-files
			sl.Debugf("ignoring unknown line format '%s'", line)
			continue
		}
		v = strings.TrimSpace(v)

		switch k {
		case "Package":
			e.Pkg = v
			num++
		case "Version":
			e.Version = v
		case "Architecture":
			e.Architecture = v

		case "Size":
			var err error
			e.Size, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("error parsing size '%s': %s", v, err)
			}
		case "SHA256":
			h, err := digests.NewSha256Digest(v)
			if err != nil {
				return 0, fmt.Errorf("error parsing digest '%s': %s", v, err)
			}
			entries[h] = e
		}
	}

	return num, nil
}

func (ins *AptPackagesInspector) addPackages(origin, packagesPath string, data *aptPackages, sl logger.Logger) {
	ins.packagesLock.Lock()
	defer ins.packagesLock.Unlock()

	if ins.packages[origin] == nil {
		ins.packages[origin] = map[string]*aptPackages{}
	}

	sl.Debugf("adding packages origin %q, %q", origin, packagesPath)
	ins.packages[origin][packagesPath] = data
}

func (ins *AptPackagesInspector) getOriginAlias(origin, cfgName string, sl logger.Logger) (string, bool) {
	repos, ok := ins.config.Repositories[cfgName]
	if !ok {
		sl.Debugf("%s: repository configuration not found", cfgName)
		return "", false
	}

	alias := repos.BaseURLAlias
	if alias == "" {
		sl.Debugf("origin not aliased: %s", origin)
		return origin, true
	}

	sl.Debugf("packages origin alias: %s to %s", origin, alias)
	return alias, true
}

// validateDebianPackage verifies if the deb package is listed in the
// Packages.xz file. The package downloaded from the package pool.
func (ins *AptPackagesInspector) validateDebianPackage(f ArtifactReader, a ResponseArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	sl := a.Logger()

	md := ArtifactMetadata{Type: mimetypes.DebianBinaryPackage}

	cfgName, ok := a.RequestStringAnnotation(ins.ID(), "cfg-name")
	if !ok {
		a.SetResponseRejected(ins, "deb file downloaded from unknown repository", md)
		return nil
	}
	origin, ok := ins.getOriginAlias(utils.NormalizedOrigin(u), cfgName, sl)
	if !ok {
		a.SetResponseRejected(ins, "Unknown repository configuration name", md).Annotate(
			Annotation{"cfg-name": cfgName},
		)
		return nil
	}

	info, err := apt_cfg.NewDebPackageURLInfo(u, &ins.config, sl)
	if err != nil {
		return fmt.Errorf("invalid deb package URL")
	}

	// check this deb against the packages file we know
	for _, pkg := range ins.packages[origin] {
		entry, ok := pkg.getPackagesEntry(a.Sha256())
		if ok {
			packagesIsValid := ins.isValidPackages(pkg.sha256)

			notes := Annotation{
				"packages-name":         entry.Pkg,           // package name in the Packages file
				"packages-version":      entry.Version,       // package version in the Packages file
				"packages-architecture": entry.Architecture,  // package architecture in the Packages file
				"packages-size":         entry.Size,          // package size in the Packages file
				"packages-file":         pkg.sha256.String(), // digest of the validating Packages file
				"packages-is-valid":     packagesIsValid,     // packages file validated against Release
				"dist":                  pkg.suite,           // suite from packages file
				"component":             info.Component,      // component from URL
			}

			// Check if the packages file listing this deb was validated
			md.Name = entry.Pkg
			md.Version = entry.Version
			md.Architecture = entry.Architecture
			md.AptSuite = pkg.suite
			if !packagesIsValid {
				a.SetResponseRejected(ins, "artifact listed in invalid Packages file", md).Annotate(notes)
			} else if a.Size() != entry.Size {
				a.SetResponseRejected(ins, "artifact size does not match Packages entry", md).Annotate(notes)
			} else if info.Architecture != entry.Architecture {
				a.SetResponseRejected(ins, "URL architecture does not match Packages entry", md).Annotate(notes)
			} else {
				a.SetResponseUnknown(ins, "deb file matches packages entry", md).Annotate(notes)
			}

			return nil
		}
	}

	a.SetResponseRejected(ins, "deb file digest not listed in packages file", ArtifactMetadata{Type: mimetypes.DebianBinaryPackage})
	return nil
}
