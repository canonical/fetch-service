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

package metadata

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/xi2/xz"
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

func aptReleaseDetector(raw []byte, limit uint32) bool {
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

type aptReleaseInspector struct{}

func (aptReleaseInspector) Inspect(filename string, md *Metadata, di *DownloadInfo, ctx *InspectionContext) (stop bool, err error) {
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
					log.Printf("warning: error parsing '%s'", line)
					continue
				}
				p.Vendor = md.Vendor
				digest, p.Path = fields[0], fields[2]
				p.Size, err = strconv.ParseInt(fields[1], 10, 64)
				if err != nil {
					log.Printf("warning: error parsing size '%s': %s", fields[1], err)
					continue
				}
				ctx.AddReleasePackages(md.Sha1, digest, p)
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

func aptLegacyReleaseDetector(raw []byte, limit uint32) bool {
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Split(bufio.ScanLines)

	for sc.Scan() {
		line := sc.Text()

		k, _, ok := strings.Cut(line, ": ")
		if !ok {
			return false
		}

		switch k {
		case "Archive", "Version", "Component", "Origin", "Label", "Architecture":
			continue
		default:
			return false
		}

	}

	return true
}

type aptLegacyReleaseInspector struct{}

func (aptLegacyReleaseInspector) Inspect(filename string, md *Metadata, di *DownloadInfo, ctx *InspectionContext) (stop bool, err error) {
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

	contents := AnnotationDetails{}

	for sc.Scan() {
		line := sc.Text()

		k, v, ok := strings.Cut(line, ": ")
		if !ok {
			return
		}

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

	md.Annotate(Notice, "apt.metadata.release", di.URL).SetDetails(contents)

	return
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

func aptPackagesDetector(raw []byte, limit uint32) bool {
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

type aptPackagesInspector struct{}

func (aptPackagesInspector) Inspect(filename string, md *Metadata, di *DownloadInfo, ctx *InspectionContext) (stop bool, err error) {
	if md.Type != "application/x-apt-packages" {
		return
	}
	stop = true

	// obtain the Packages.xz path from the Release file
	relDigest, p, ok := ctx.GetReleasePackages(md.Sha256)
	if ok {
		if p.Size != md.Size {
			data := AnnotationDetails{"release-data": p}
			md.Annotate(IntegrityViolation, "file.integrity.check", ResultFail).SetDetails(data)
			return
		}
		// The Packages file is listed in Release and size matches
		md.Annotate(Notice, "file.integrity.asserted-by", relDigest)
	} else {
		// This Packages file was not found in InRelease
		md.Annotate(PolicyViolation, "file.integrity.check", ResultFail)
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
			ctx.AddPackagesEntry(md.Sha1, v, e)
		}
	}

	md.Annotate(Notice, "apt.metadata.packages.count", strconv.Itoa(num))

	return
}
