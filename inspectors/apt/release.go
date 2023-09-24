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
	sd    SessionDetails
	state AptReleaseContext
}

func (ins *AptReleaseInspector) ID() string {
	return "apt.release"
}

func (ins *AptReleaseInspector) InitializeContext(sd SessionDetails) {
	ins.sd = sd
	ins.state = AptReleaseContext{
		releasePackages: make(map[metadata.Sha256Digest]map[metadata.Sha256Digest]AptReleasePackages, 16),
	}
}

func (ins *AptReleaseInspector) InspectRequest(a *metadata.Artefact) error {
	return nil
}

func (ins *AptReleaseInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) (stop bool, err error) {
	if a.Metadata.Type != mimetypes.AptRelease {
		return
	}
	stop = true

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
				p.Vendor = a.Metadata.Vendor
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
				ins.state.AddReleasePackages(a.Metadata.Sha256, h, p)
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

	a.Metadata.Name = "InRelease"

	return
}

func (ins *AptReleaseInspector) API() InspectorAPI {
	return AptReleaseInspectorAPI(&ins.state)
}

type AptReleaseInspectorAPI interface {
	InspectorAPI
	ValidatePackagesFile(int64, metadata.Sha256Digest) error
	GetReleasePackages(metadata.Sha256Digest) (metadata.Sha256Digest, AptReleasePackages, bool)
}

func GetAptReleaseInspectorAPI(sd SessionDetails) (AptReleaseInspectorAPI, error) {
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
