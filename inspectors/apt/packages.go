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
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/xi2/xz"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
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
	r, err := xz.NewReader(bytes.NewReader(raw), 0)
	if err != nil {
		return false
	}

	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil {
		return false
	}

	buf = buf[:n]

	sc := bufio.NewScanner(bytes.NewReader(buf))
	sc.Split(bufio.ScanLines)

	fields := map[string]struct{}{}

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

		fields[k] = struct{}{}
	}

	expected_fields := []string{
		"Package",
		"Architecture",
		"Version",
		"Priority",
		"Section",
		"Origin",
		"Maintainer",
	}

	for _, k := range expected_fields {
		_, ok := fields[k]
		if !ok {
			logger.Debugf("expected field %q not found", k)
			return false // we don't recognize this file
		}
	}

	return true
}

// aptPackagesEntry stores selected fields from each package listed in
// each downloaded Packages.* file.
type aptPackagesEntry struct {
	pkg          string // package name
	architecture string // package architecture
	version      string // package version
	size         int64  // package size in bytes
}

// aptPackages holds information about the Packages.xz file.
type aptPackages struct {
	sha256       metadata.Sha256Digest
	origin       string // URL origin of the archive
	distribution string // name of the distribution
	component    string // name of the component
	architecture string

	entries     map[metadata.Sha256Digest]aptPackagesEntry
	entriesLock sync.Mutex
}

func newAptPackages(origin, distribution, component, architecture string) *aptPackages {
	return &aptPackages{
		origin:       origin,
		distribution: distribution,
		component:    component,
		architecture: architecture,

		entries: make(map[metadata.Sha256Digest]aptPackagesEntry),
	}
}

// addPackagesEntry adds package information the inspector state.
func (pkg *aptPackages) addPackagesEntry(digest metadata.Sha256Digest, e aptPackagesEntry) {
	pkg.entriesLock.Lock()
	defer pkg.entriesLock.Unlock()

	pkg.entries[digest] = e
}

// getPackagesEntry retrieves package information from the inspector state.
func (pkg *aptPackages) getPackagesEntry(digest metadata.Sha256Digest) (aptPackagesEntry, bool) {
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
}

func NewAptPackagesInspector() *AptPackagesInspector {
	return &AptPackagesInspector{
		packages: make(map[string]map[string]*aptPackages),
	}
}

func (ins *AptPackagesInspector) ID() string {
	return "apt.packages"
}

func (ins *AptPackagesInspector) InspectRequest(a *metadata.Artefact) error {
	u, err := url.Parse(a.CurrentDownload.URL)
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if info, err := newPackagesUrlInfo(u); err == nil {
		a.Hold(ins, "valid URL for Packages file").Annotate(
			metadata.Annotation{
				"origin":       info.origin,
				"repository":   info.repository,
				"distribution": info.distribution,
				"component":    info.component,
				"architecture": info.architecture,
			},
		)
		packages := newAptPackages(info.origin, info.distribution, info.component, info.architecture)
		ins.addPackages(info.origin, u.Path, packages)
	} else if info, err := newDebPackageUrlInfo(u); err == nil {
		a.Hold(ins, "valid URL for deb package").Annotate(
			metadata.Annotation{
				"origin":       info.origin,
				"repository":   info.repository,
				"component":    info.component,
				"name":         info.name,
				"version":      info.version,
				"architecture": info.architecture,
			},
		)
	}

	return nil
}

func (ins *AptPackagesInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
	if a.Metadata.Type == mimetypes.DebianBinaryPackage {
		return ins.validateDebianPackage(f, a)
	}

	if a.Metadata.Type != mimetypes.AptPackages {
		return nil
	}

	u, err := url.Parse(a.CurrentDownload.URL)
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	origin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	pkg, ok := ins.getPackages(origin, u.Path)
	if !ok {
		return fmt.Errorf("inconsistent package state: %q, %q", origin, u.Path)
	}
	pkg.sha256 = a.Metadata.Sha256

	// Add packages list to inspector state
	r, err := xz.NewReader(f, 0)
	if err != nil {
		return err
	}

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
			return fmt.Errorf("error parsing line '%s'", line)
		}
		v = strings.TrimSpace(v)

		switch k {
		case "Package":
			e.pkg = v
			num++
		case "Version":
			e.version = v
		case "Architecture":
			e.architecture = v
		case "Size":
			var err error
			e.size, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("error parsing size '%s': %s", v, err)
			}
		case "SHA256":
			var h metadata.Sha256Digest
			h, err = metadata.NewSha256Digest(v)
			if err != nil {
				return fmt.Errorf("error parsing digest '%s': %s", v, err)
			}
			pkg.addPackagesEntry(h, e)
		}
	}

	a.Metadata.Name = "Packages.xz"
	a.Metadata.Version = pkg.distribution
	a.Metadata.Description = fmt.Sprintf("%s %s Packages file",
		pkg.distribution, pkg.component)
	a.Metadata.Architecture = pkg.architecture

	// the file should be also annotated by the release inspector
	rins, ok := a.ResponseInspection["apt.release"]
	if ok {
		v, ok := rins.Annotations["vendor"]
		if ok {
			a.Metadata.Author = fmt.Sprintf("%s", v)
			a.Metadata.Vendor = fmt.Sprintf("%s", v)
		}
	}

	a.Approve(ins, "packages file succesfully parsed").Annotate(
		metadata.Annotation{
			"package-count": num,
		},
	)

	return nil
}

func (ins *AptPackagesInspector) addPackages(origin, packagesPath string, data *aptPackages) {
	ins.packagesLock.Lock()
	defer ins.packagesLock.Unlock()

	if ins.packages[origin] == nil {
		ins.packages[origin] = map[string]*aptPackages{}
	}
	logger.Debugf("adding packages origin %q, %q", origin, packagesPath)
	ins.packages[origin][packagesPath] = data
}

func (ins *AptPackagesInspector) getPackages(origin, packagesPath string) (*aptPackages, bool) {
	ins.packagesLock.Lock()
	defer ins.packagesLock.Unlock()

	logger.Debugf("retrieving packages origin %q, %q", origin, packagesPath)
	pkg, ok := ins.packages[origin]
	if !ok {
		logger.Warningf("package origin %q is unknown", origin)
		return nil, false
	}
	data, ok := pkg[packagesPath]
	if !ok {
		logger.Warningf("package path %q is unknown", packagesPath)
		return nil, false
	}

	return data, true
}

// validateDebianPackage verifies if the deb package is listed in the
// Packages.xz file. The package downloaded from the package pool.
func (ins *AptPackagesInspector) validateDebianPackage(f ReadAtSeeker, a *metadata.Artefact) error {

	u, err := url.Parse(a.CurrentDownload.URL)
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}
	origin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	info, err := newDebPackageUrlInfo(u)
	if err != nil {
		return fmt.Errorf("invalid deb package URL")
	}

	// check this deb against the packages file we know
	for _, pkg := range ins.packages[origin] {
		entry, ok := pkg.getPackagesEntry(a.Metadata.Sha256)
		if ok {
			notes := metadata.Annotation{
				"packages-name":         entry.pkg,           // package name in the Packages file
				"packages-version":      entry.version,       // package version in the Packages file
				"packages-architecture": entry.architecture,  // package architecture in the Packages file
				"packages-size":         entry.size,          // package size in the Packages file
				"packages-file":         pkg.sha256.String(), // digest of the validating Packages file
				"distribution":          pkg.distribution,    // distribution from packages file
				"component":             info.component,      // component from URL
			}
			if a.Metadata.Size != entry.size {
				a.Reject(ins, "artefact size does not match Packages entry").Annotate(notes)
			} else if info.architecture != entry.architecture {
				a.Reject(ins, "URL architecture does not match Packages entry").Annotate(notes)
			} else {
				a.Approve(ins, "deb file matches packages entry").Annotate(notes)
			}
			return nil
		}
	}

	a.Reject(ins, "deb file digest not listed in packages file")
	return nil
}
