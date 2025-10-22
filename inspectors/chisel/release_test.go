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

package chisel_test

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"sort"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/glob"
	apt_cfg "github.com/canonical/fetch-service/inspectors/apt/config"
	"github.com/canonical/fetch-service/inspectors/chisel"
	"github.com/canonical/fetch-service/inspectors/chisel/config"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/testutils"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

func getTestChiselConfig() config.ChiselInspectorConfig {
	return config.ChiselInspectorConfig{
		Urls: []glob.Glob{
			glob.MustCompile("https://codeload.github.com:443/canonical/chisel-releases/**"),
		},
	}
}

var ubuntuArchivePubKey = testutils.PGPKeys["key-ubuntu-2018"]

func getTestAptConfig() apt_cfg.AptInspectorConfig {
	return apt_cfg.AptInspectorConfig{
		Repositories: map[string]apt_cfg.AptInspectorConfigRepository{
			"default": {
				Urls:       []glob.Glob{glob.MustCompile("http://*.ubuntu.com/ubuntu")},
				Suites:     []glob.Glob{glob.MustCompile("focal")},
				Components: []glob.Glob{glob.MustCompile("main")},
				PublicKey:  ubuntuArchivePubKey.PubKeyArmor,
			},
		},
	}
}

func (s *chiselSuite) TestChiselReleaseInspectorID(c *C) {
	cfg := getTestChiselConfig()
	aptCfg := getTestAptConfig()
	ins := chisel.NewChiselReleaseInspector(cfg, aptCfg)
	c.Assert(ins.ID(), Equals, "chisel.release")
}

type releaseInspectRequestTest struct {
	url     string
	opinion opinions.OpinionKind
}

var releaseInspectRequestTests = []releaseInspectRequestTest{{
	url:     "https://codeload.github.com:443/canonical/chisel-releases/tar.gz/refs/heads/ubuntu-22.04",
	opinion: opinions.Pending,
}, {
	url:     "http://unknown.location/foo",
	opinion: opinions.Unknown,
}}

func (s *chiselSuite) TestChiselReleaseInspectRequest(c *C) {
	cfg := getTestChiselConfig()
	aptCfg := getTestAptConfig()

	for _, test := range releaseInspectRequestTests {
		ins := chisel.NewChiselReleaseInspector(cfg, aptCfg)

		a := metadata.NewArtifact()
		a.CurrentDownload.URL = test.url

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil) // We do not expect any errors.

		switch test.opinion {
		case opinions.Pending, opinions.Rejected:
			insp, ok := a.RequestInspection[ins.ID()]
			c.Assert(ok, Equals, true) // Must have opinion.
			c.Assert(insp.Opinion, Equals, test.opinion)
		default:
			insp, ok := a.RequestInspection[ins.ID()]
			if ok {
				c.Assert(insp.Opinion, Equals, test.opinion)
			}
		}
	}
}

type chiselReleaseArtifactTest struct {
	summary  string
	files    map[string]string // files to compress to a tar.gz file.
	mimetype string            // set artifact type to mimetype.
	approved bool
	metadata metadata.Metadata
	err      string
	rootDirs []string
	tar      bool
}

var chiselReleaseArtifactTests = []chiselReleaseArtifactTest{{
	summary: "Valid archive",
	files: map[string]string{
		"chisel.yaml": "testdata/chisel.yaml.sample",
	},
	mimetype: "application/gzip",
	metadata: metadata.Metadata{
		Type:        "application/x.canonical.chisel-release",
		Name:        "chisel-release",
		Version:     "v1",
		Description: "Chisel release file for ubuntu-20.04",
		Vendor:      "Canonical",
	},
	tar:      true,
	approved: true,
}, {
	summary:  "Missing files: chisel.yaml",
	files:    map[string]string{},
	mimetype: "application/gzip",
	tar:      true,
}, {
	summary: "Invalid chisel.yaml: missing fields",
	files: map[string]string{
		"chisel.yaml": "testdata/chisel.yaml.invalid.missing",
	},
	mimetype: "application/gzip",
	tar:      true,
	err:      "invalid tarball: invalid chisel.yaml: missing fields",
}, {
	summary: "Invalid chisel.yaml: missing fields in archive",
	files: map[string]string{
		"chisel.yaml": "testdata/chisel.yaml.invalid.archive.missing",
	},
	mimetype: "application/gzip",
	tar:      true,
	err:      `invalid tarball: invalid chisel.yaml: archive "ubuntu" has missing fields`,
}, {
	summary: "Invalid chisel.yaml: undefined pubkey in archive",
	files: map[string]string{
		"chisel.yaml": "testdata/chisel.yaml.invalid.archive.pubkey-undefined",
	},
	mimetype: "application/gzip",
	tar:      true,
	err:      `invalid tarball: invalid chisel.yaml: archive "ubuntu" pubkey "foo" undefined`,
}, {
	summary: "Invalid chisel.yaml: missing fields in pubkey",
	files: map[string]string{
		"chisel.yaml": "testdata/chisel.yaml.invalid.pubkey.missing",
	},
	mimetype: "application/gzip",
	tar:      true,
	err:      `invalid tarball: invalid chisel.yaml: pubkey "ubuntu-archive-key-2018" has missing fields`,
}, {
	summary: "Invalid chisel.yaml: pubkey not present in APT config",
	files: map[string]string{
		"chisel.yaml": "testdata/chisel.yaml.invalid.pubkey-absent-in-apt-config",
	},
	mimetype: "application/gzip",
	tar:      true,
	err:      `invalid public-keys: invalid chisel.yaml: no public key is present in APT configuration`,
}, {
	summary: "Invalid chisel.yaml: key data contains multiple keys",
	files: map[string]string{
		"chisel.yaml": "testdata/chisel.yaml.invalid.multi-keys",
	},
	mimetype: "application/gzip",
	tar:      true,
	err:      `invalid public-keys: cannot parse chisel.yaml public key ubuntu-archive-key-2018: armored data contains more than one public key`,
}, {
	summary: "Invalid mimetype",
	files: map[string]string{
		"chisel.yaml": "testdata/chisel.yaml.invalid.missing",
	},
	mimetype: "text/plain",
	tar:      true,
	err:      "", // We do not recognize this artifact.
}, {
	summary: "Unknown top-level dir",
	files: map[string]string{
		"chisel.yaml": "testdata/chisel.yaml.sample",
	},
	mimetype: "application/gzip",
	rootDirs: []string{"another-dir"},
	tar:      true,
	err:      "", // We do not recognize this artifact.
}, {
	summary:  "Gzip is not of a tar archive",
	mimetype: "application/gzip",
	err:      "", // We do not recognize this artifact.
	tar:      false,
}}

func (s *chiselSuite) TestChiselReleaseInspectArtifact(c *C) {
	const release = "ubuntu-20.04"
	const rootDir = "chisel-releases-" + release

	cfg := getTestChiselConfig()
	aptCfg := getTestAptConfig()

	for _, test := range chiselReleaseArtifactTests {
		c.Logf("Summary: %s", test.summary)

		rootDirs := []string{}
		rootDirs = append(rootDirs, test.rootDirs...)
		rootDirs = append(rootDirs, rootDir)

		ins := chisel.NewChiselReleaseInspector(cfg, aptCfg)

		// Prepare artifact.
		a := metadata.NewArtifact()
		a.Metadata.Type = test.mimetype
		inspection := &Inspection{
			Opinion: opinions.Pending,
			Reason:  "some reason",
		}
		a.RequestInspection[ins.ID()] = inspection

		// Create the artifact file.
		var filename string
		var err error
		if test.tar {
			filename, err = createTarGz(test.files, rootDirs)
		} else {
			filename, err = createPhonyGz()
		}
		c.Assert(err, IsNil)
		defer os.Remove(filename)

		f, err := files.OpenArtifactFile(filename)
		c.Assert(err, IsNil)
		defer f.Close()

		err = ins.InspectArtifact(f, a)

		c.Assert(err, IsNil)

		if test.err != "" {
			insp := a.ResponseInspection[ins.ID()]
			c.Assert(insp.Opinion, Equals, opinions.Rejected)
			c.Assert(insp.Reason, Equals, test.err)
			continue
		}

		c.Assert(a.Approved(), Equals, test.approved)
		if test.approved {
			c.Check(a.Metadata, DeepEquals, test.metadata)
		}
	}
}

func createPhonyGz() (string, error) {
	f, err := os.CreateTemp("", "not-tar*.gz")
	if err != nil {
		return "", err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	if _, err = gw.Write([]byte{1, 2, 3}); err != nil {
		return "", err
	}

	return f.Name(), nil
}

// Creates a tar.gz archive with the [files]. The archive should have the last directory
// in [roots] contain the [files]. It returns the path of the created file.
func createTarGz(files map[string]string, roots []string) (string, error) {
	writeDir := func(w *tar.Writer, dir string) error {
		if !strings.HasSuffix(dir, "/") {
			dir += "/"
		}
		return w.WriteHeader(&tar.Header{
			Typeflag: tar.TypeDir,
			Name:     dir,
			Mode:     0755,
		})
	}
	writeFile := func(w *tar.Writer, dest, src string) error {
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(stat, stat.Name())
		if err != nil {
			return err
		}
		hdr.Name = dest

		if err := w.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		return err
	}

	f, err := os.CreateTemp("", "chisel-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, root := range roots {
		if err := writeDir(tw, root); err != nil {
			return "", err
		}
	}

	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, dest := range keys {
		src := files[dest]
		dest = roots[len(roots)-1] + "/" + dest

		if strings.HasSuffix(dest, "/") {
			if err := writeDir(tw, dest); err != nil {
				return "", err
			}
			continue
		}

		if err := writeFile(tw, dest, src); err != nil {
			return "", err
		}
	}
	return f.Name(), nil
}
