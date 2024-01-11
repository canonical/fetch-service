// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023 Canonical Ltd.
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
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/xi2/xz"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
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

	score := 0

	for sc.Scan() {
		line := sc.Text()

		k, _, ok := strings.Cut(line, ": ")
		if !ok {
			return false
		}

		switch k {
		case "Package", "Architecture", "Version", "Priority", "Section", "Origin", "Maintainer":
			score++
			if score == 7 {
				return true
			}
			continue
		case "Source": // optional
			continue
		default:
			return false
		}
	}

	return true
}

// aptPackagesEntry stores selected fields from each package listed in
// each downloaded Packages.* file.
type aptPackagesEntry struct {
	Package      string
	Version      string
	Architecture string
	Size         int64
}

// AptPackagesInspector contains inspector-specific contextual data for stateful
// analysis within a fetch session.
type AptPackagesInspector struct {
	// packagesEntries maps Packages.* file digests to package digest to metadata.
	packagesEntries map[metadata.Sha256Digest]map[metadata.Sha256Digest]aptPackagesEntry
	packagesLock    sync.Mutex
}

func NewAptPackagesInspector() *AptPackagesInspector {
	return &AptPackagesInspector{
		packagesEntries: make(map[metadata.Sha256Digest]map[metadata.Sha256Digest]aptPackagesEntry),
	}
}

func (ins *AptPackagesInspector) ID() string {
	return "apt.packages"
}

func (ins *AptPackagesInspector) InspectRequest(a *metadata.Artefact) error {
	validReqs := []*regexp.Regexp{
		regexp.MustCompile(`http://archive\.ubuntu\.com/`),
		regexp.MustCompile(`http://security\.ubuntu\.com/`),
		regexp.MustCompile(`https://esm\.ubuntu\.com:443/`),
		regexp.MustCompile(`http://repo.ros2.org/`),
	}

	for _, re := range validReqs {
		if re.MatchString(a.CurrentDownload.URL) {
			a.ApproveRequest(ins)
			return nil
		}
	}
	return nil // we don't recognize this request
}

func (ins *AptPackagesInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
	if a.Metadata.Type == mimetypes.DebianBinaryPackage {
		ins.validateDebianPackage(f, a)
		return nil
	}

	if a.Metadata.Type != mimetypes.AptPackages {
		return nil
	}

	// Cache some data to check deb packages
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
			e.Package = v
			num++
		case "Version":
			e.Version = v
		case "Architecture":
			e.Architecture = v
		case "Size":
			e.Size, _ = strconv.ParseInt(v, 10, 32)
		case "SHA256":
			var h metadata.Sha256Digest
			h, err = metadata.NewSha256Digest(v)
			if err != nil {
				return fmt.Errorf("error parsing digest '%s': %s", v, err)
			}
			ins.addPackagesEntry(a.Metadata.Sha256, h, e)
		}
	}

	a.Approve(ins, "%d packages parsed", num)

	return nil
}

func (ins *AptPackagesInspector) addPackagesEntry(pkgsDigest metadata.Sha256Digest, digest metadata.Sha256Digest, e aptPackagesEntry) {
	ins.packagesLock.Lock()
	defer ins.packagesLock.Unlock()

	if ins.packagesEntries[pkgsDigest] == nil {
		ins.packagesEntries[pkgsDigest] = make(map[metadata.Sha256Digest]aptPackagesEntry)
	}
	ins.packagesEntries[pkgsDigest][digest] = e
}

func (ins *AptPackagesInspector) getPackagesEntry(digest metadata.Sha256Digest) (metadata.Sha256Digest, aptPackagesEntry, bool) {
	ins.packagesLock.Lock()
	defer ins.packagesLock.Unlock()

	for d, entries := range ins.packagesEntries {
		if e, ok := entries[digest]; ok {
			return d, e, ok
		}
	}
	return metadata.Sha256Digest{}, aptPackagesEntry{}, false
}

func (ins *AptPackagesInspector) validateDebianPackage(f ReadAtSeeker, a *metadata.Artefact) {
	digest, e, ok := ins.getPackagesEntry(a.Metadata.Sha256)
	if ok {
		md := a.Metadata
		if md.Name == e.Package && md.Version == e.Version && md.Architecture == e.Architecture && md.Size == e.Size {
			a.Approve(ins, "deb file validated by packages file %s", digest)
		} else {
			a.Reject(ins, "deb file metadata does not match packages file %s", digest)
		}
	} else {
		a.Reject(ins, "deb file digest not listed in packages file")
	}
}
