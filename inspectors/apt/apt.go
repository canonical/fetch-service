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
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/xi2/xz"

	"github.com/canonical/fetch-service/inspectors/api"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
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
// MD5Sum:
// ...
// 7b01f6f56157ccab98fa819e9b68ec6c           240952 main/binary-amd64/Packages
// 9b5dcc779cf96d4238ffcb081973d1de            49419 main/binary-amd64/Packages.gz
// 43289ba88c740edc6b69860b6c428eb9            40928 main/binary-amd64/Packages.xz
// 19d98231d41ff3fb09d4f260df38e5ac              105 main/binary-amd64/Release
// ...

func AptReleaseDetector(raw []byte, limit uint32) bool {
	b := bytes.NewReader(raw)
	sc := bufio.NewScanner(b)
	sc.Split(bufio.ScanLines)

	score := 0

	for sc.Scan() {
		line := sc.Text()

		if len(line) == 0 || line[0] == ' ' {
			continue
		}

		k, _, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		// These are used in the Ubuntu archives, however according
		// to the specs some of them are optional. May need to run
		// better checks to handle third-party repos. See
		// https://wiki.debian.org/DebianRepository/Format#A.22Release.22_files
		switch k {
		case "Origin", "Label", "Suite", "Version", "Codename", "Date",
			"Components", "Architectures", "Description":
			score++
		case "MD5Sum", "SHA1", "SHA256":
			return score > 7
		}
	}

	return false
}

// AptReleasePackages stores information about each Packages.* file listed
// in the repository's Release file.
type AptReleasePackages struct {
	Vendor string // repository vendor
	Path   string // path to the Packages file
	Size   int64  // size of the Packages file
}

// AptReleaseContext contains inspector-specific contextual data for stateful
// analysis within a fetch session.
type AptReleaseContext struct {
	// releasePackages maps InRelease file digests to Packages.* file digests to metadata.
	releasePackages map[metadata.Sha256Digest]map[metadata.Sha256Digest]AptReleasePackages
	releaseLock     sync.Mutex
}

func (ctx *AptReleaseContext) ValidatePackagesFile(size int64, digest metadata.Sha256Digest) error {
	return nil
}

func (ctx *AptReleaseContext) AddReleasePackages(relDigest metadata.Sha256Digest, digest metadata.Sha256Digest, p AptReleasePackages) {
	ctx.releaseLock.Lock()
	defer ctx.releaseLock.Unlock()

	if ctx.releasePackages[relDigest] == nil {
		ctx.releasePackages[relDigest] = make(map[metadata.Sha256Digest]AptReleasePackages, 16)
	}
	ctx.releasePackages[relDigest][digest] = p
	logger.Debugf("apt releases file: %s %s", digest, p.Path)
}

func (ctx *AptReleaseContext) GetReleasePackages(digest metadata.Sha256Digest) (relDigest metadata.Sha256Digest, p AptReleasePackages, ok bool) {
	ctx.releaseLock.Lock()
	defer ctx.releaseLock.Unlock()

	for d, pkgs := range ctx.releasePackages {
		p, ok = pkgs[digest]
		if ok {
			relDigest = d
			return
		}
	}
	return
}

type AptReleaseInspector struct {
	sd    api.SessionDetails
	state AptReleaseContext
}

func (ins *AptReleaseInspector) Name() string {
	return "apt.release"
}

func (ins *AptReleaseInspector) InitializeContext(sd api.SessionDetails) {
	ins.sd = sd
	ins.state = AptReleaseContext{
		releasePackages: make(map[metadata.Sha256Digest]map[metadata.Sha256Digest]AptReleasePackages, 16),
	}
}

func (ins *AptReleaseInspector) AuthorizeDownload(di *metadata.DownloadInfo) error {
	return nil
}

func (ins *AptReleaseInspector) Inspect(filename string, md *metadata.Metadata, di *metadata.DownloadInfo) (stop bool, err error) {
	if md.Type != "application/x-apt-release" {
		return
	}
	stop = true

	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Split(bufio.ScanLines)

	hashes := false

	var p AptReleasePackages

	for sc.Scan() {
		line := sc.Text()

		if line == "-----BEGIN PGP SIGNATURE-----" {
			break
		}

		var digest string

		// Get sha256 hashes for Package.xz files
		if hashes && len(line) > 0 && line[0] == ' ' {
			if strings.HasSuffix(line, "/Packages.xz") {
				p = AptReleasePackages{}
				fields := strings.Fields(line)
				if len(fields) != 3 {
					logger.Warningf("cannot parse '%s'", line)
					continue
				}
				p.Vendor = md.Vendor
				digest, p.Path = fields[0], fields[2]
				p.Size, err = strconv.ParseInt(fields[1], 10, 64)
				if err != nil {
					logger.Warningf("cannot parse '%s': %s", fields[1], err)
					continue
				}
				h, err := metadata.NewSha256Digest(digest)
				if err != nil {
					logger.Warningf("cannot parse digest '%s': %s", digest, err)
					continue
				}
				ins.state.AddReleasePackages(md.Sha256, h, p)
			}
			continue
		}

		hashes = false

		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)

		switch k {
		case "Origin":
			md.Vendor = v
			md.Author = v
		case "Version":
			md.Version = v
		case "Description":
			md.Description = v
		case "SHA256":
			hashes = true
		}
	}

	md.Name = "InRelease"

	return
}

func (ins *AptReleaseInspector) API() api.InspectorAPI {
	return AptReleaseInspectorAPI(&ins.state)
}

type AptReleaseInspectorAPI interface {
	api.InspectorAPI
	ValidatePackagesFile(int64, metadata.Sha256Digest) error
	GetReleasePackages(metadata.Sha256Digest) (metadata.Sha256Digest, AptReleasePackages, bool)
}

func GetAptReleaseInspectorAPI(sd api.SessionDetails) (AptReleaseInspectorAPI, error) {
	res, err := sd.GetInspectorAPI("apt.release")
	if err != nil {
		return nil, err
	}

	api, ok := res.(AptReleaseInspectorAPI)
	if !ok {
		return nil, fmt.Errorf("cannot get ApiReleaseInsepctorAPI instance")
	}

	return api, nil
}

// Per-component Release file
// (http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/Release)
//
// Example content:
//
// Archive: jammy
// Version: 22.04
// Component: main
// Origin: Ubuntu
// Label: Ubuntu
// Architecture: amd64

func AptLegacyReleaseDetector(raw []byte, limit uint32) bool {
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Split(bufio.ScanLines)

	matches := 0

	for sc.Scan() {
		line := sc.Text()
		if strings.Count(line, ":") != 1 {
			return false
		}

		k, _, ok := strings.Cut(line, ":")
		if !ok {
			return false
		}

		switch k {
		case "Archive", "Version", "Component", "Origin", "Label", "Architecture":
			matches++
			continue
		default:
			return false
		}

	}

	return matches == 6
}

type AptLegacyReleaseInspector struct{}

func (AptLegacyReleaseInspector) Inspect(filename string, md *metadata.Metadata, di *metadata.DownloadInfo) (stop bool, err error) {
	if md.Type != "application/x-apt-legacy-release" {
		return
	}
	stop = true

	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Split(bufio.ScanLines)

	var component string
	var architecture string

	contents := metadata.AnnotationValue{}

	for sc.Scan() {
		line := sc.Text()

		k, v, ok := strings.Cut(line, ":")
		if !ok {
			return
		}
		v = strings.TrimSpace(v)

		contents[k] = v

		switch k {
		case "Origin":
			md.Vendor = v
			md.Author = v
		case "Version":
			md.Version = v
		case "Component":
			component = v
		case "Architecture":
			architecture = v
		}
	}
	md.Name = "Release"
	md.Description = fmt.Sprintf("Repository release file for %s/%s", component, architecture)

	md.Annotate("apt.metadata.release", contents)

	return
}

func (ins *AptLegacyReleaseInspector) API() api.InspectorAPI {
	return nil
}

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

// AptPackagesEntry stores selected fields from each package listed in
// each downloaded Packages.* file.
type AptPackagesEntry struct {
	Package      string
	Version      string
	Architecture string
	Size         int64
}

// AptPackagesContext contains inspector-specific contextual data for stateful
// analysis within a fetch session.
type AptPackagesContext struct {
	sd api.SessionDetails

	// packagesEntries maps Packages.* file digests to package digest to metadata.
	packagesEntries map[metadata.Sha256Digest]map[metadata.Sha256Digest]AptPackagesEntry
	packagesLock    sync.Mutex
}

func (ctx *AptPackagesContext) ValidateDebFile(size int64, digest metadata.Sha256Digest) error {
	return nil
}

func (ctx *AptPackagesContext) AddPackagesEntry(pkgsDigest metadata.Sha256Digest, digest metadata.Sha256Digest, e AptPackagesEntry) {
	ctx.packagesLock.Lock()
	defer ctx.packagesLock.Unlock()

	if ctx.packagesEntries[pkgsDigest] == nil {
		ctx.packagesEntries[pkgsDigest] = make(map[metadata.Sha256Digest]AptPackagesEntry)
	}
	ctx.packagesEntries[pkgsDigest][digest] = e
}

func (ctx *AptPackagesContext) GetPackagesEntry(digest metadata.Sha256Digest) (pkgsDigest metadata.Sha256Digest, e AptPackagesEntry, ok bool) {
	ctx.packagesLock.Lock()
	defer ctx.packagesLock.Unlock()

	for d, entries := range ctx.packagesEntries {
		e, ok = entries[digest]
		if ok {
			pkgsDigest = d
			return
		}
	}
	return
}

type AptPackagesInspector struct {
	sd    api.SessionDetails
	state AptPackagesContext
}

func (ins *AptPackagesInspector) Name() string {
	return "apt.packages"
}

func (ins *AptPackagesInspector) InitializeContext(sd api.SessionDetails) {
	ins.sd = sd
	ins.state = AptPackagesContext{
		packagesEntries: make(map[metadata.Sha256Digest]map[metadata.Sha256Digest]AptPackagesEntry, 256),
	}
}

func (ins *AptPackagesInspector) AuthorizeDownload(di *metadata.DownloadInfo) error {
	return nil
}

func (ins *AptPackagesInspector) Inspect(filename string, md *metadata.Metadata, di *metadata.DownloadInfo) (stop bool, err error) {
	if md.Type != "application/x-apt-packages" {
		return
	}
	stop = true

	release, err := GetAptReleaseInspectorAPI(ins.sd)
	if err != nil {
		logger.Error("internal error: cannot get apt release API")
		return
	}

	// obtain the Packages.xz path from the Release file
	relDigest, p, ok := release.GetReleasePackages(md.Sha256)
	if ok {
		/*
			if p.Size != md.Size {
				data := AnnotationDetails{"release-data": p}
				md.Annotate(IntegrityViolation, "file.integrity.check", ResultFail).SetDetails(data)
				return
			}
		*/
		// The Packages file is listed in Release and size matches
		md.Annotate("apt.packages.integrity.asserted-by", metadata.AnnotationValue{"release-file": relDigest.String()})
	} else {
		// This Packages file was not found in InRelease
		md.Annotate("apt.packages.integrity.fail", metadata.AnnotationValue{})
	}

	// Populate metadata
	md.Name = p.Path
	md.Vendor = p.Vendor
	md.Description = "Apt repository Packages file"
	md.Author = p.Vendor

	// Cache some data to check deb packages
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	r, err := xz.NewReader(f, 0)
	if err != nil {
		return
	}

	sc := bufio.NewScanner(r)
	sc.Split(bufio.ScanLines)

	// some lines can be really long (e.g. librust-winapi-dev Provides:)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	var e AptPackagesEntry

	num := 0

	for sc.Scan() {
		line := sc.Text()

		if line == "" {
			e = AptPackagesEntry{}
			continue
		}

		k, v, ok := strings.Cut(line, ":")
		if !ok {
			err = fmt.Errorf("error parsing line '%s'", line)
			return
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
				err = fmt.Errorf("error parsing digest '%s': %s", v, err)
				return
			}
			ins.state.AddPackagesEntry(md.Sha256, h, e)
		}
	}

	//md.Annotate("apt.metadata.packages.count", strconv.Itoa(num))

	return
}

func (ins *AptPackagesInspector) API() api.InspectorAPI {
	return AptPackagesInspectorAPI(&ins.state)
}

type AptPackagesInspectorAPI interface {
	api.InspectorAPI
	ValidateDebFile(int64, metadata.Sha256Digest) error
}
