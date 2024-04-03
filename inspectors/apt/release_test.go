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

package apt_test

import (
	"errors"
	"io"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/apt"
	"github.com/canonical/fetch-service/metadata"
)

// XXX: This file contains minimal testing for apt file formats. Tests
//      will be extended after the metadata format is approved.

var inReleaseArtefactData = `-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA512

Origin: Ubuntu
Label: Ubuntu
Suite: jammy-backports
Version: 22.04
Codename: jammy
Date: Fri, 07 Jul 2023 18:13:42 UTC
Architectures: amd64 arm64 armhf i386 ppc64el riscv64 s390x
Components: main restricted universe multiverse
Description: Ubuntu Jammy Backports
NotAutomatic: yes
ButAutomaticUpgrades: yes
MD5Sum:
 7b01f6f56157ccab98fa819e9b68ec6c           240952 main/binary-amd64/Packages
 9b5dcc779cf96d4238ffcb081973d1de            49419 main/binary-amd64/Packages.gz
 43289ba88c740edc6b69860b6c428eb9            40928 main/binary-amd64/Packages.xz
SHA1:
 b7c117896a538ceb37a99da6fb7981511b2524fa           240952 main/binary-amd64/Packages
 7eacb20b664866781cd506321cc0138d1f87570e            49419 main/binary-amd64/Packages.gz
 16234a62a83c8e2bf9c6a5f38acd22f06b3002c5            40928 main/binary-amd64/Packages.xz
SHA256:
 65183fe1e5a4f9881147fdd0042dfa259fb2fca0e86b57457e74e507358c63b6           240952 main/binary-amd64/Packages
 3b2b1ad6f76bec3c692d5932ceffed8c3c261c8b5fde78cd084432352c83d14d            49419 main/binary-amd64/Packages.gz
 9efc4736be7bf5aa4ca05f28af96dc58f8491b488c930cf2c40f67e71d69beb6            40928 main/binary-amd64/Packages.xz
Acquire-By-Hash: yes
-----BEGIN PGP SIGNATURE-----
Version: GnuPG v1
 
-----END PGP SIGNATURE-----`

func (s *aptSuite) TestAptReleaseArtefactInspector(c *C) {
	for _, tc := range []struct {
		data     string
		validSig bool
		result   bool
	}{
		{inReleaseArtefactData, true, true},
		{inReleaseArtefactData, false, false},
		{"some arbitrary data", true, false},
	} {
		restorer := apt.MockCheckSignature(func(f io.ReadSeeker, notes metadata.Annotation) (io.ReadSeeker, error) {
			if !tc.validSig {
				return f, errors.New("invalid signature")
			}

			return f, nil
		})
		defer restorer()

		a := metadata.NewArtefact()
		a.CurrentDownload.URL = "https://my.archive/test"
		a.MimeType = mimetype.Lookup("text/plain")

		f := strings.NewReader(tc.data)

		ins := apt.NewAptReleaseInspector()
		err := ins.InspectArtefact(f, a)
		c.Assert(err, IsNil)

		c.Assert(a.Approved(), Equals, tc.result)

		if tc.result {
			c.Check(a.Metadata.Type, Equals, "application/x.apt.release")
			c.Check(a.Metadata.Name, Equals, "InRelease")
			c.Check(a.Metadata.Vendor, Equals, "Ubuntu")
			c.Check(a.Metadata.Description, Equals, "Ubuntu Jammy Backports")
			c.Check(a.Metadata.Author, Equals, "Ubuntu")
			c.Check(a.ResponseInspection["apt.release"].Annotations, DeepEquals, metadata.Annotation{
				"Architectures":        "amd64 arm64 armhf i386 ppc64el riscv64 s390x",
				"ButAutomaticUpgrades": "yes",
				"Codename":             "jammy",
				"Components":           "main restricted universe multiverse",
				"Date":                 "Fri, 07 Jul 2023 18:13:42 UTC",
				"Description":          "Ubuntu Jammy Backports",
				"Hash":                 "SHA512",
				"Label":                "Ubuntu",
				"NotAutomatic":         "yes",
				"Origin":               "Ubuntu",
				"Suite":                "jammy-backports",
				"Version":              "22.04",
			})

			sha256_1, _ := metadata.NewSha256Digest("65183fe1e5a4f9881147fdd0042dfa259fb2fca0e86b57457e74e507358c63b6")
			sha256_2, _ := metadata.NewSha256Digest("3b2b1ad6f76bec3c692d5932ceffed8c3c261c8b5fde78cd084432352c83d14d")
			sha256_3, _ := metadata.NewSha256Digest("9efc4736be7bf5aa4ca05f28af96dc58f8491b488c930cf2c40f67e71d69beb6")

			// verify internal state
			state := ins.Release()
			c.Check(state["https://my.archive/test"], DeepEquals, apt.ReleaseFile{
				Vendor: "Ubuntu",
				Files: map[metadata.Sha256Digest]apt.ReleaseEntry{
					sha256_1: apt.ReleaseEntry{Name: "main/binary-amd64/Packages", Size: 240952},
					sha256_2: apt.ReleaseEntry{Name: "main/binary-amd64/Packages.gz", Size: 49419},
					sha256_3: apt.ReleaseEntry{Name: "main/binary-amd64/Packages.xz", Size: 40928},
				},
			})
		}
	}

}
