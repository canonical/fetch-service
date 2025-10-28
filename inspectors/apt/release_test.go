// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2024 Canonical Ltd.
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
	"bytes"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/xi2/xz"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/inspectors/apt"
	apt_cfg "github.com/canonical/fetch-service/inspectors/apt/config"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
)

var inReleaseArtifactData = `-----BEGIN PGP SIGNED MESSAGE-----
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
 4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed              792 main/i18n/Translation-zh_TW.xz
Acquire-By-Hash: yes
-----BEGIN PGP SIGNATURE-----
Version: GnuPG v1

-----END PGP SIGNATURE-----`

var inReleaseArtifactMetaData = metadata.Metadata{
	Type:        "application/x.apt.release",
	Name:        "InRelease",
	Vendor:      "Ubuntu",
	Description: "Ubuntu Jammy Backports",
	Author:      "Ubuntu",
	Version:     "jammy",
	AptSuite:    "jammy-backports",
}

var inReleaseArtifactAnnotation = Annotation{
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
}

func getTestAptConfig() apt_cfg.AptInspectorConfig {
	return apt_cfg.AptInspectorConfig{
		Repositories: map[string]apt_cfg.AptInspectorConfigRepository{
			"default": {
				URLs:       []glob.Glob{glob.MustCompile("http://archive.ubuntu.com/ubuntu")},
				Suites:     []glob.Glob{glob.MustCompile("jammy")},
				Components: []glob.Glob{glob.MustCompile("main")},
				PublicKey:  publicKey,
			},
			"aliased": {
				URLs:         []glob.Glob{glob.MustCompile("http://notalias.ubuntu.com/**")},
				Suites:       []glob.Glob{glob.MustCompile("noble")},
				Components:   []glob.Glob{glob.MustCompile("main")},
				PublicKey:    publicKey,
				BaseURLAlias: "http://alias.ubuntu.com",
			},
		},
	}
}

type releaseArtifactInspectorTest struct {
	data       string
	metadata   metadata.Metadata
	annotation Annotation
	validSig   bool
	result     bool
}

var releaseArtifactInspectorTests = []releaseArtifactInspectorTest{{
	data:       inReleaseArtifactData,
	metadata:   inReleaseArtifactMetaData,
	annotation: inReleaseArtifactAnnotation,
	validSig:   true,
	result:     true,
}, {
	data:       inReleaseArtifactData,
	metadata:   inReleaseArtifactMetaData,
	annotation: inReleaseArtifactAnnotation,
	validSig:   false,
	result:     false,
}, {
	data:       "some arbitrary data",
	metadata:   metadata.Metadata{},
	annotation: Annotation{},
	validSig:   true,
	result:     false,
}}

func (s *aptSuite) TestAptReleaseArtifactInspector(c *C) {
	// Create data without the optional "Description" field.
	artifactDataNoDesc := strings.ReplaceAll(
		inReleaseArtifactData,
		"Description: Ubuntu Jammy Backports\n", "",
	)
	artifactMetaDataNoDesc := inReleaseArtifactMetaData
	artifactMetaDataNoDesc.Description = "Ubuntu jammy-backports"
	artifactAnnotationNoDesc := Annotation{}
	for k, v := range inReleaseArtifactAnnotation {
		if k != "Description" {
			artifactAnnotationNoDesc[k] = v
		}
	}

	tests := releaseArtifactInspectorTests
	tests = append(tests, releaseArtifactInspectorTest{
		data:       artifactDataNoDesc,
		metadata:   artifactMetaDataNoDesc,
		annotation: artifactAnnotationNoDesc,
		validSig:   true,
		result:     true,
	})

	for _, tc := range tests {
		restorer := apt.MockCheckSignature(func(f io.ReadSeeker, notes Annotation, pubkey string) (io.ReadSeeker, error) {
			if !tc.validSig {
				return f, errors.New("invalid signature")
			}

			return f, nil
		})
		defer restorer()

		a := metadata.NewArtifact()
		a.RequestInspection = metadata.InspectionMap{
			"apt.release": &Inspection{
				Opinion: opinions.Pending,
				Reason:  "",
				Annotations: Annotation{
					"cfg-name": "default",
				},
			},
		}
		a.CurrentDownload.URL = "https://my.archive/test"
		a.MimeType = mimetype.Lookup("text/plain")

		f := strings.NewReader(tc.data)

		ins := apt.NewAptReleaseInspector(getTestAptConfig())
		err := ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Assert(a.Approved(), Equals, tc.result)

		if tc.result {
			c.Check(a.Metadata, DeepEquals, tc.metadata)
			c.Check(a.ResponseInspection["apt.release"].Annotations, DeepEquals, tc.annotation)

			sha256_1, _ := digests.NewSha256Digest("65183fe1e5a4f9881147fdd0042dfa259fb2fca0e86b57457e74e507358c63b6")
			sha256_2, _ := digests.NewSha256Digest("3b2b1ad6f76bec3c692d5932ceffed8c3c261c8b5fde78cd084432352c83d14d")
			sha256_3, _ := digests.NewSha256Digest("9efc4736be7bf5aa4ca05f28af96dc58f8491b488c930cf2c40f67e71d69beb6")
			sha256_4, _ := digests.NewSha256Digest("4970d559683cafc299958246973f62fb75edbccf8cbbf67f6b3a7d05982e44ed")

			// verify internal state
			state := ins.Release()
			c.Check(state["https://my.archive/test"], DeepEquals, apt.ReleaseFile{
				Vendor: "Ubuntu",
				Files: map[digests.Sha256Digest]apt.ReleaseEntry{
					sha256_1: apt.ReleaseEntry{Name: "main/binary-amd64/Packages", Size: 240952},
					sha256_2: apt.ReleaseEntry{Name: "main/binary-amd64/Packages.gz", Size: 49419},
					sha256_3: apt.ReleaseEntry{Name: "main/binary-amd64/Packages.xz", Size: 40928},
					sha256_4: apt.ReleaseEntry{Name: "main/i18n/Translation-zh_TW.xz", Size: 792},
				},
			})
		}
	}
}

type aptReleasePackagesValidationTest struct {
	url         string               // Current download URL
	cfgName     string               // The repository entry in configuration
	opinion     opinions.OpinionKind // This inspector's opinion
	reason      string               // The reason for the inspector's opinion
	releaseRepo string               // The repository name in the release state
}

var aptReleasePackagesValidationTests = []aptReleasePackagesValidationTest{{
	url:         "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/by-hash/SHA256/65183fe1e5a4f9881147fdd0042dfa259fb2fca0e86b57457e74e507358c63b6",
	cfgName:     "default",
	opinion:     opinions.Unknown,
	reason:      "Packages file listed in Release",
	releaseRepo: "http://archive.ubuntu.com/ubuntu/dists/jammy",
}, {
	url:         "http://notalias.ubuntu.com/ubuntu/dists/noble/main/binary-amd64/by-hash/SHA256/65183fe1e5a4f9881147fdd0042dfa259fb2fca0e86b57457e74e507358c63b6",
	cfgName:     "aliased",
	opinion:     opinions.Unknown,
	reason:      "Packages file listed in Release",
	releaseRepo: "http://alias.ubuntu.com/ubuntu/dists/noble",
}, {
	cfgName:     "", // missing cfg-name
	url:         "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/by-hash/SHA256/65183fe1e5a4f9881147fdd0042dfa259fb2fca0e86b57457e74e507358c63b6",
	opinion:     opinions.Rejected,
	reason:      "Packages file downloaded from unknown repository",
	releaseRepo: "http://archive.ubuntu.com/ubuntu/dists/jammy",
}, {
	cfgName:     "invalid", // invalid cfg-name
	url:         "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/by-hash/SHA256/65183fe1e5a4f9881147fdd0042dfa259fb2fca0e86b57457e74e507358c63b6",
	opinion:     opinions.Rejected,
	reason:      "Unknown repository configuration name",
	releaseRepo: "http://archive.ubuntu.com/ubuntu/dists/jammy",
}}

func (s *aptSuite) TestAptReleasePackagesValidation(c *C) {
	for _, tc := range aptReleasePackagesValidationTests {
		sha256_rel, err := digests.NewSha256Digest("9efc4736be7bf5aa4ca05f28af96dc58f8491b488c930cf2c40f67e71d69beb6")
		c.Assert(err, IsNil)
		sha256_pkg, err := digests.NewSha256Digest("65183fe1e5a4f9881147fdd0042dfa259fb2fca0e86b57457e74e507358c63b6")
		c.Assert(err, IsNil)

		a := metadata.NewArtifact()
		a.CurrentDownload.URL = tc.url
		a.Metadata.Type = mimetypes.AptPackages
		a.Metadata.Sha256 = sha256_pkg

		f := strings.NewReader("fake content")

		rf := apt.ReleaseFile{
			Sha256: sha256_rel,
			Vendor: "Canonical",
			Files: map[digests.Sha256Digest]apt.ReleaseEntry{
				sha256_pkg: {
					Size: 1337,
					Name: "main/binary-amd64/Packages.xz",
				},
			},
		}

		ins := apt.NewAptReleaseInspector(getTestAptConfig())
		notes := Annotation{}
		if tc.cfgName != "" {
			notes["cfg-name"] = tc.cfgName
		}
		a.SetRequestPending(ins, "test").Annotate(notes)
		ins.SetRelease(map[string]apt.ReleaseFile{tc.releaseRepo: rf})
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Assert(a.Approved(), Equals, false)
		if tc.opinion == opinions.Unknown {
			c.Assert(a.ResponseInspection["apt.release"], DeepEquals, &Inspection{
				Opinion: tc.opinion,
				Reason:  tc.reason,
				Annotations: Annotation{
					"file-path":    "main/binary-amd64/Packages.xz",
					"release-file": "9efc4736be7bf5aa4ca05f28af96dc58f8491b488c930cf2c40f67e71d69beb6",
					"vendor":       "Canonical",
				},
			})
		} else {
			c.Assert(a.ResponseInspection["apt.release"].Opinion, Equals, tc.opinion)
			c.Assert(a.ResponseInspection["apt.release"].Reason, Equals, tc.reason)
		}
	}
}

type aptReleaseTranslationValidationTest struct {
	url         string               // Current download URL
	cfgName     string               // The repository entry in configuration
	opinion     opinions.OpinionKind // This inspector's opinion
	reason      string               // The reason for the inspector's opinion
	releaseRepo string               // The repository name in the release state
}

var aptReleaseTranslationValidationTests = []aptReleaseTranslationValidationTest{{
	url:         "http://archive.ubuntu.com/ubuntu/dists/jammy/main/i18n/by-hash/SHA256/65183fe1e5a4f9881147fdd0042dfa259fb2fca0e86b57457e74e507358c63b6",
	cfgName:     "default",
	opinion:     opinions.Unknown,
	reason:      "Translation file listed in Release",
	releaseRepo: "http://archive.ubuntu.com/ubuntu/dists/jammy",
}, {
	url:         "http://notalias.ubuntu.com/ubuntu/dists/noble/main/i18n/by-hash/SHA256/65183fe1e5a4f9881147fdd0042dfa259fb2fca0e86b57457e74e507358c63b6",
	cfgName:     "aliased",
	opinion:     opinions.Unknown,
	reason:      "Translation file listed in Release",
	releaseRepo: "http://alias.ubuntu.com/ubuntu/dists/noble",
}, {
	url:         "http://archive.ubuntu.com/ubuntu/dists/jammy/main/i18n/by-hash/SHA256/65183fe1e5a4f9881147fdd0042dfa259fb2fca0e86b57457e74e507358c63b6",
	cfgName:     "", // missing cfg-name
	opinion:     opinions.Rejected,
	reason:      "Translation file downloaded from unknown repository",
	releaseRepo: "http://archive.ubuntu.com/ubuntu/dists/jammy",
}, {
	url:         "http://archive.ubuntu.com/ubuntu/dists/jammy/main/i18n/by-hash/SHA256/65183fe1e5a4f9881147fdd0042dfa259fb2fca0e86b57457e74e507358c63b6",
	cfgName:     "invalid", // invalid cfg-name
	opinion:     opinions.Rejected,
	reason:      "Unknown repository configuration name",
	releaseRepo: "http://archive.ubuntu.com/ubuntu/dists/jammy",
}}

func (s *aptSuite) TestAptReleaseTranslationValidation(c *C) {
	for _, tc := range aptReleaseTranslationValidationTests {
		sha256_rel, err := digests.NewSha256Digest("9efc4736be7bf5aa4ca05f28af96dc58f8491b488c930cf2c40f67e71d69beb6")
		c.Assert(err, IsNil)
		sha256_trn, err := digests.NewSha256Digest("65183fe1e5a4f9881147fdd0042dfa259fb2fca0e86b57457e74e507358c63b6")
		c.Assert(err, IsNil)

		a := metadata.NewArtifact()
		a.CurrentDownload.URL = tc.url
		a.Metadata.Type = mimetypes.AptTranslation
		a.Metadata.Sha256 = sha256_trn
		a.Metadata.Size = 1337

		f := strings.NewReader("fake content")

		rf := apt.ReleaseFile{
			Sha256: sha256_rel,
			Vendor: "Canonical",
			Files: map[digests.Sha256Digest]apt.ReleaseEntry{
				sha256_trn: apt.ReleaseEntry{
					Size: 1337,
					Name: "main/i18n/Translation-en.xz",
				},
			},
		}

		ins := apt.NewAptReleaseInspector(getTestAptConfig())
		notes := Annotation{}
		if tc.cfgName != "" {
			notes["cfg-name"] = tc.cfgName
		}
		a.SetRequestPending(ins, "test").Annotate(notes)
		ins.SetRelease(map[string]apt.ReleaseFile{tc.releaseRepo: rf})
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Assert(a.Approved(), Equals, false)
		if tc.opinion == opinions.Unknown {
			c.Assert(a.ResponseInspection["apt.release"], DeepEquals, &Inspection{
				Opinion: tc.opinion,
				Reason:  tc.reason,
				Annotations: Annotation{
					"file-path":    "main/i18n/Translation-en.xz",
					"release-file": "9efc4736be7bf5aa4ca05f28af96dc58f8491b488c930cf2c40f67e71d69beb6",
					"vendor":       "Canonical",
				},
			})
		} else {
			c.Assert(a.ResponseInspection["apt.release"].Opinion, Equals, tc.opinion)
			c.Assert(a.ResponseInspection["apt.release"].Reason, Equals, tc.reason)
		}
	}
}

type aptReleaseCommandsValidationTest struct {
	url         string               // Current download URL
	cfgName     string               // The repository entry in configuration
	opinion     opinions.OpinionKind // This inspector's opinion
	reason      string               // The reason for the inspector's opinion
	releaseRepo string               // The repository name in the release state
	isListed    bool                 // Whether the Commands file is listed in the Release file.
}

var aptReleaseCommandsValidationTests = []aptReleaseCommandsValidationTest{{
	url:         "http://archive.ubuntu.com/ubuntu/dists/jammy/main/cnf/by-hash/SHA256/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
	cfgName:     "default",
	opinion:     opinions.Unknown,
	reason:      "Commands file listed in Release",
	releaseRepo: "http://archive.ubuntu.com/ubuntu/dists/jammy",
	isListed:    true,
}, {
	url:         "http://archive.ubuntu.com/ubuntu/dists/jammy/main/cnf/by-hash/SHA256/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
	cfgName:     "default",
	opinion:     opinions.Rejected,
	reason:      "Commands file not listed in Release file",
	releaseRepo: "http://archive.ubuntu.com/ubuntu/dists/jammy",
	isListed:    false,
}, {
	url:         "http://notalias.ubuntu.com/ubuntu/dists/noble/main/cnf/by-hash/SHA256/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
	cfgName:     "aliased",
	opinion:     opinions.Unknown,
	reason:      "Commands file listed in Release",
	releaseRepo: "http://alias.ubuntu.com/ubuntu/dists/noble",
	isListed:    true,
}, {
	url:         "http://archive.ubuntu.com/ubuntu/dists/jammy/main/cnf/by-hash/SHA256/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
	cfgName:     "", // missing cfg-name
	opinion:     opinions.Rejected,
	reason:      "Commands file downloaded from unknown repository",
	releaseRepo: "http://alias.ubuntu.com/ubuntu/dists/jammy",
	isListed:    true,
}, {
	url:         "http://archive.ubuntu.com/ubuntu/dists/jammy/main/cnf/by-hash/SHA256/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
	cfgName:     "invalid", // invalid cfg-name
	opinion:     opinions.Rejected,
	reason:      "Unknown repository configuration name",
	releaseRepo: "http://alias.ubuntu.com/ubuntu/dists/jammy",
	isListed:    true,
}}

func (s *aptSuite) TestAptReleaseCommandsValidation(c *C) {
	for _, tc := range aptReleaseCommandsValidationTests {
		sha256_rel, err := digests.NewSha256Digest("9efc4736be7bf5aa4ca05f28af96dc58f8491b488c930cf2c40f67e71d69beb6")
		c.Assert(err, IsNil)
		sha256_cmd, err := digests.NewSha256Digest("6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf")
		c.Assert(err, IsNil)

		a := metadata.NewArtifact()
		a.CurrentDownload.URL = tc.url
		a.Metadata.Type = mimetypes.AptCommands
		a.Metadata.Sha256 = sha256_cmd
		a.Metadata.Size = 1337

		f := strings.NewReader("fake content")
		rf := apt.ReleaseFile{
			Sha256: sha256_rel,
			Vendor: "Canonical",
			Files:  map[digests.Sha256Digest]apt.ReleaseEntry{},
		}

		if tc.isListed {
			rf.Files[sha256_cmd] = apt.ReleaseEntry{
				Size: 1337,
				Name: "main/cnf/Commands-amd64.xz",
			}
		}

		ins := apt.NewAptReleaseInspector(getTestAptConfig())
		notes := Annotation{}
		if tc.cfgName != "" {
			notes["cfg-name"] = tc.cfgName
		}
		a.SetRequestPending(ins, "test").Annotate(notes)
		ins.SetRelease(map[string]apt.ReleaseFile{tc.releaseRepo: rf})
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		c.Assert(a.Approved(), Equals, false)
		if tc.opinion == opinions.Unknown {
			c.Assert(a.ResponseInspection["apt.release"], DeepEquals, &Inspection{
				Opinion: tc.opinion,
				Reason:  tc.reason,
				Annotations: Annotation{
					"file-path":    "main/cnf/Commands-amd64.xz",
					"release-file": "9efc4736be7bf5aa4ca05f28af96dc58f8491b488c930cf2c40f67e71d69beb6",
					"vendor":       "Canonical",
				},
			})
		} else {
			c.Assert(a.ResponseInspection["apt.release"].Opinion, Equals, tc.opinion)
			c.Assert(a.ResponseInspection["apt.release"].Reason, Equals, tc.reason)
		}
	}
}

func (s *aptSuite) TestAptReleaseSignature(c *C) {
	a := metadata.NewArtifact()
	a.RequestInspection = metadata.InspectionMap{
		"apt.release": &Inspection{
			Opinion: opinions.Pending,
			Reason:  "",
			Annotations: Annotation{
				"cfg-name": "default",
			},
		},
	}
	a.CurrentDownload.URL = "https://archive.ubuntu.com/ubuntu/dists/jammy/InRelease"
	a.MimeType = mimetype.Lookup("text/plain")

	f, err := os.Open("testdata/InRelease.xz")
	c.Assert(err, IsNil)
	defer f.Close()

	// Read compressed file
	z, err := xz.NewReader(f, 0)
	c.Assert(err, IsNil)

	buf := bytes.NewBuffer([]byte{})
	_, err = io.Copy(buf, z)
	c.Assert(err, IsNil)
	r := bytes.NewReader(buf.Bytes())

	ins := apt.NewAptReleaseInspector(getTestAptConfig())
	err = ins.InspectArtifact(r, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, true)

	c.Check(a.Metadata.Type, Equals, "application/x.apt.release")
	c.Check(a.Metadata.Name, Equals, "InRelease")
	c.Check(a.Metadata.Vendor, Equals, "Ubuntu")
	c.Check(a.Metadata.Description, Equals, "Ubuntu Jammy Updates")
	c.Check(a.Metadata.Author, Equals, "Ubuntu")
	c.Check(a.ResponseInspection["apt.release"].Annotations, DeepEquals, Annotation{
		"Architectures": "amd64 arm64 armhf i386 ppc64el riscv64 s390x",
		"Codename":      "jammy",
		"Components":    "main restricted universe multiverse",
		"Date":          "Fri, 03 May 2024 21:17:09 UTC",
		"Description":   "Ubuntu Jammy Updates",
		"Label":         "Ubuntu",
		"Origin":        "Ubuntu",
		"Suite":         "jammy-updates",
		"Version":       "22.04",
		"public-keys":   []string{"871920D1991BC93C"},
	})
}
