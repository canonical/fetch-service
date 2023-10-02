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
	"regexp"
	"strconv"
	"strings"
	"sync"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
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

// aptReleasePackages stores information about each Packages.* file listed
// in the repository's Release file.
type aptReleasePackages struct {
	Vendor string // repository vendor
	Path   string // path to the Packages file
	Size   int64  // size of the Packages file
}

// AptReleaseInspector contains inspector-specific contextual data for stateful
// analysis within a fetch session.
type AptReleaseInspector struct {
	// releasePackages maps InRelease file digests to Packages.* file digests to metadata.
	releasePackages map[metadata.Sha256Digest]map[metadata.Sha256Digest]aptReleasePackages
	releaseLock     sync.Mutex
}

func NewAptReleaseInspector() *AptReleaseInspector {
	return &AptReleaseInspector{
		releasePackages: make(map[metadata.Sha256Digest]map[metadata.Sha256Digest]aptReleasePackages),
	}
}

func (ins *AptReleaseInspector) ID() string {
	return "apt.release"
}

func (ins *AptReleaseInspector) InspectRequest(a *metadata.Artefact) error {
	validReqs := []*regexp.Regexp{
		regexp.MustCompile(`http://archive\.ubuntu\.com/`),
		regexp.MustCompile(`http://security\.ubuntu\.com/`),
		regexp.MustCompile(`https://esm\.ubuntu\.com:443/`),
		regexp.MustCompile(`http://repo.ros2.org/`),
	}

	for _, re := range validReqs {
		if re.MatchString(a.CurrentDownload.URL) {
			a.AuthorizeRequest(ins)
		}
	}

	return nil // we don't recognize this request
}

func (ins *AptReleaseInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
	if a.Metadata.Type == mimetypes.AptPackages {
		ins.validatePackagesFile(f, a)
		return nil
	}

	if a.Metadata.Type != mimetypes.AptRelease {
		return nil
	}

	sc := bufio.NewScanner(f)
	sc.Split(bufio.ScanLines)

	hashes := false

	var p aptReleasePackages

	for sc.Scan() {
		line := sc.Text()

		if line == "-----BEGIN PGP SIGNATURE-----" {
			break
		}

		var digest string

		// Get sha256 hashes for Package.xz files
		if hashes && len(line) > 0 && line[0] == ' ' {
			if strings.HasSuffix(line, "/Packages.xz") {
				p = aptReleasePackages{}
				fields := strings.Fields(line)
				if len(fields) != 3 {
					logger.Warningf("cannot parse '%s'", line)
					continue
				}
				p.Vendor = a.Metadata.Vendor
				digest, p.Path = fields[0], fields[2]

				size, err := strconv.ParseInt(fields[1], 10, 64)
				if err != nil {
					logger.Warningf("cannot parse '%s': %s", fields[1], err)
					continue
				}
				p.Size = size

				h, err := metadata.NewSha256Digest(digest)
				if err != nil {
					logger.Warningf("cannot parse digest '%s': %s", digest, err)
					continue
				}
				ins.addReleasePackages(a.Metadata.Sha256, h, p)
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
			a.Metadata.Vendor = v
			a.Metadata.Author = v
		case "Version":
			a.Metadata.Version = v
		case "Description":
			a.Metadata.Description = v
		case "SHA256":
			hashes = true
		}
	}

	a.Approve(ins, "release file parse successful")

	a.Metadata.Name = "InRelease"

	return nil
}

func (ins *AptReleaseInspector) addReleasePackages(relDigest metadata.Sha256Digest, digest metadata.Sha256Digest, p aptReleasePackages) {
	ins.releaseLock.Lock()
	defer ins.releaseLock.Unlock()

	if ins.releasePackages[relDigest] == nil {
		ins.releasePackages[relDigest] = make(map[metadata.Sha256Digest]aptReleasePackages, 16)
	}
	ins.releasePackages[relDigest][digest] = p
	logger.Debugf("apt releases file: %s %s", digest, p.Path)
}

func (ins *AptReleaseInspector) getReleasePackages(digest metadata.Sha256Digest) (metadata.Sha256Digest, aptReleasePackages, bool) {
	ins.releaseLock.Lock()
	defer ins.releaseLock.Unlock()

	for d, pkgs := range ins.releasePackages {
		if p, ok := pkgs[digest]; ok {
			return d, p, ok
		}
	}
	return metadata.Sha256Digest{}, aptReleasePackages{}, false
}

func (ins *AptReleaseInspector) validatePackagesFile(f ReadAtSeeker, a *metadata.Artefact) {
	digest, pinfo, ok := ins.getReleasePackages(a.Metadata.Sha256)
	if ok {
		a.Approve(ins, "packages file %s listed in release file", digest)
	} else {
		a.Reject(ins, "packages file digest not listed in release file")
	}
	a.Metadata.Vendor = pinfo.Vendor
}
