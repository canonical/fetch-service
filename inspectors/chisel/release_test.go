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

	"github.com/canonical/fetch-service/inspectors/chisel"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

func (s *chiselSuite) TestChiselReleaseInspectorID(c *C) {
	ins := chisel.NewChiselReleaseInspector()
	c.Assert(ins.ID(), Equals, "chisel.release")
}

func (s *chiselSuite) TestChiselReleaseInspectRequest(c *C) {
	const baseURL = "https://codeload.github.com:443/canonical/chisel-releases/tar.gz/refs/heads"

	for _, test := range []struct {
		url     string
		opinion opinions.OpinionKind
	}{
		{baseURL + "/ubuntu-22.04", opinions.Pending},
		{"http://unknown.location/foo", opinions.Unknown},
	} {
		ins := chisel.NewChiselReleaseInspector()

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
	err      string
}

var chiselReleaseArtifactTests = []chiselReleaseArtifactTest{{
	summary: "Valid archive",
	files: map[string]string{
		"chisel.yaml": "testdata/chisel.yaml.sample",
		"slices/":     "",
	},
	mimetype: "application/gzip",
	approved: true,
}, {
	summary: "Missing files: slices directory",
	files: map[string]string{
		"chisel.yaml": "testdata/chisel.yaml.sample",
	},
	mimetype: "application/gzip",
}, {
	summary: "Missing files: chisel.yaml",
	files: map[string]string{
		"slices/": "",
	},
	mimetype: "application/gzip",
}, {
	summary: "Missing files: chisel.yaml",
	files: map[string]string{
		"slices/": "",
	},
	mimetype: "application/gzip",
}, {
	summary: "Invalid chisel.yaml",
	files: map[string]string{
		"chisel.yaml": "testdata/chisel.yaml.invalid",
		"slices/":     "",
	},
	mimetype: "application/gzip",
	err:      "cannot parse chisel.yaml: invalid chisel.yaml",
}, {
	summary: "Invalid mimetype",
	files: map[string]string{
		"chisel.yaml": "testdata/chisel.yaml.invalid",
		"slices/":     "",
	},
	mimetype: "text/plain",
}}

func (s *chiselSuite) TestChiselReleaseInspectArtifact(c *C) {
	const branch = "ubuntu-24.04"
	const rootDir = "chisel-releases-" + branch

	for _, test := range chiselReleaseArtifactTests {
		c.Logf("Summary: %s", test.summary)

		ins := chisel.NewChiselReleaseInspector()

		// Prepare artifact.
		a := metadata.NewArtifact()
		a.Metadata.Type = test.mimetype
		inspection := &Inspection{
			Opinion: opinions.Pending,
			Reason:  "some reason",
		}
		inspection.Annotate(Annotation{
			"branch-name": branch,
		})
		a.RequestInspection[ins.ID()] = inspection

		// Create the artifact file.
		filename, err := createTarGz(test.files, rootDir)
		c.Assert(err, IsNil)
		defer os.Remove(filename)

		f, err := files.OpenArtifactFile(filename)
		c.Assert(err, IsNil)
		defer f.Close()

		err = ins.InspectArtifact(f, a)
		if test.err != "" {
			c.Assert(err, ErrorMatches, test.err)
			continue
		}
		c.Assert(err, IsNil)

		c.Assert(a.Approved(), Equals, test.approved)
		if test.approved {
			c.Check(a.Metadata.Type, Equals, mimetypes.ChiselRelease)
			c.Check(a.Metadata.Name, Equals, branch)
			c.Check(a.Metadata.Version, Matches, `^v[1-9][0-9]*$`)
		}
	}
}

// Creates a tar.gz archive with the [files]. The archive should have the [root]
// directory containing the files. It returns the path of the created file.
func createTarGz(files map[string]string, root string) (string, error) {
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

	if err := writeDir(tw, root); err != nil {
		return "", err
	}

	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, dest := range keys {
		src := files[dest]
		dest = root + "/" + dest

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
