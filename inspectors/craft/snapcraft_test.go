// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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

package craft_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/glob"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/craft"
	"github.com/canonical/fetch-service/inspectors/craft/config"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/git"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/gabriel-vasile/mimetype"
)

type snapcraftSuite struct {
	slog logger.Logger
}

var _ = Suite(&snapcraftSuite{logger.NewSessionLogger("test")})

func (t *snapcraftSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

func SnapcraftTest(t *testing.T) { TestingT(t) }

func getTestSnapcraftConfig() config.CraftsInspectorConfig {
	return config.CraftsInspectorConfig{
		URLs: []glob.Glob{
			glob.MustCompile("https://github.com:443/**"),
			glob.MustCompile("https://git.launchpad.net:443/**"),
		},
	}
}

func (s *snapcraftSuite) TestSnapcraftInspectorInterface(c *C) {
	var iface Inspector
	ins := craft.NewSnapcraftInspector(getTestSnapcraftConfig())
	c.Assert(ins, Implements, &iface)

}

func (s *snapcraftSuite) TestUploadPackInspectorID(c *C) {
	ins := craft.NewSnapcraftInspector(getTestSnapcraftConfig())
	c.Assert(ins.ID(), Equals, "craft.snapcraft")

}

func createTestSnapcraftArtifact(checkoutPath string) *metadata.Artifact {
	a := metadata.NewArtifact()
	a.Request, _ = http.NewRequest("GET", "https://example.com:443/test/git-upload-pack", nil)
	a.CurrentDownload.ContentType = "application/x-git-upload-pack-result"
	a.Request.Body = io.NopCloser(strings.NewReader("0014command=fetch\n0000"))
	a.MimeType = mimetype.Lookup("application/octet-stream")
	a.RequestInspection = metadata.InspectionMap{
		"git.upload-pack": &Inspection{
			Opinion: opinions.Pending,
			Reason:  "valid URL for snapcraft upload-pack",
			Annotations: Annotation{
				"client-request": []string{
					"command=fetch",
					"agent=git/2.45.1",
					"object-format=sha1",
					"",
					"thin-pack",
					"no-progress",
					"include-tag",
					"ofs-delta",
					"deepen 1",
					"want 9ae13d6ca5afec49279f8515feb289a7069e5a29",
					"done",
				},
				"repository": "https://github.com/lengau/uv-snap",
				"command":    "fetch",
				"project":    "astral-uv",
				"protocol":   "version=2",
				"wants": []string{
					"9ae13d6ca5afec49279f8515feb289a7069e5a29",
				},
				"is-shallow": true,
			},
		},
	}
	a.ResponseInspection = metadata.InspectionMap{
		"git.upload-pack": &Inspection{
			Opinion: opinions.Unknown,
			Reason:  "",
			Annotations: Annotation{
				"git-checkout-path": checkoutPath,
			},
		},
	}
	return a
}

func loadTestSnapcraftArtifactData() (*files.ArtifactFile, error) {
	sourcepkgFile := filepath.Join("testdata", "snapcraftpkg.raw")
	file, err := files.OpenArtifactFile(sourcepkgFile)
	return file, err
}

func (s *snapcraftSuite) TestInspectSnapcraftGitRequest(c *C) {
	for _, tc := range []struct {
		url      string
		approved bool
	}{
		{"https://github.com:443/user/project.git/git-upload-pack", true},
		{"https://git.launchpad.net:443/project/git-upload-pack", true},
		{"https://git.launchpad.net:443/~user/project/+git/project/git-upload-pack", true},
		{"https://github.com:443/user/project/git-upload-pack", true},
		{"https://invalid.com:443/user/project.git/git-upload-pack", false},
		{"http://github.com/user/project.git/git-upload-pack", false},
		{"https://gothub.com:443/user/project.git/git-upload-pack", false},
		{"ahttps://github.com:443/user/project.git/git-upload-pack", false},
		{"https://github.com:443/user/project.git/git-upload-packs", false},
		{"https://github.com:443/user/project.git/something-else", false},
		{"https://git.launchpad.com:443/project/git-upload-pack", false},
		{"https://git.lpad.net:443/~user/project/+git/project/git-upload-pack", false},
	} {
		ins := craft.NewSnapcraftInspector(getTestSnapcraftConfig())
		a := metadata.NewArtifact()
		a.CurrentDownload.URL = tc.url
		a.CurrentDownload.RequestHeader = map[string][]string{
			"Content-Type": {"application/x-git-upload-pack-request"},
			"Accept":       {"application/x-git-upload-pack-result"},
		}
		a.RequestInspection["git.upload-pack"] = &Inspection{
			Annotations: Annotation{"command": "fetch"},
		}
		a.Request, _ = http.NewRequest("GET", tc.url, nil)
		a.Request.Body = io.NopCloser(strings.NewReader("0014command=ls-refs\n0000"))

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, tc.approved, Commentf("Aproval status is wrong for '%s' (%t != %t)", tc.url, ok, tc.approved))
		if tc.approved {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		}
	}
}

func (s *snapcraftSuite) TestSnapcraftGitInspectArtifact(c *C) {
	for _, tc := range []struct {
		opinion opinions.OpinionKind
		reason  string
	}{
		{opinions.Approved, "snapcraft repository found"},
	} {
		f, err := loadTestSnapcraftArtifactData()
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		checkoutPath := c.MkDir()
		err = git.UnpackObjects(f, checkoutPath, s.slog)
		c.Assert(err, IsNil)
		err = git.Checkout(checkoutPath, "9ae13d6ca5afec49279f8515feb289a7069e5a29", s.slog)
		c.Assert(err, IsNil)

		a := createTestSnapcraftArtifact(checkoutPath)
		c.Assert(err, IsNil)

		f, err = loadTestSnapcraftArtifactData()
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		ins := craft.NewSnapcraftInspector(getTestSnapcraftConfig())
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		inspection := a.ResponseInspection["craft.snapcraft"]
		c.Assert(inspection.Opinion, Equals, tc.opinion)
		c.Assert(inspection.Reason, Equals, tc.reason)

		if tc.opinion == opinions.Approved {
			c.Check(a.Metadata.Type, Equals, "application/x.canonical.snapcraft")
			c.Check(a.Metadata.Name, Equals, "astral-uv")
			c.Check(a.Metadata.Version, Equals, "0.4.20")
			c.Check(a.Metadata.Description, Equals, "An extremely fast Python package installer and resolver, written in Rust.")
			c.Check(a.Metadata.ContentID, Equals, "9ae13d6ca5afec49279f8515feb289a7069e5a29")
		}
	}
}

func (s *snapcraftSuite) TestSnapcraftGitInspectArtifactMissingSnapcraftYaml(c *C) {
	a := createTestSnapcraftArtifact(c.MkDir())
	f, err := loadTestSnapcraftArtifactData()
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	ins := craft.NewSnapcraftInspector(getTestSnapcraftConfig())

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	_, ok := a.ResponseInspection["craft.snapcraft"]
	c.Assert(ok, Equals, false)
}

func (s *snapcraftSuite) TestSnapcraftGitInspectArtifactUnreadableSnapcraftYaml(c *C) {
	tc := struct {
		opinion opinions.OpinionKind
		reason  string
	}{
		opinions.Rejected,
		"cannot open snapcraft.yaml file",
	}

	f, err := loadTestSnapcraftArtifactData()
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	restorer := craft.MockOsOpen(func(string) (*os.File, error) {
		return nil, os.ErrNotExist
	})
	defer restorer()

	checkoutPath := c.MkDir()
	_, err = os.Create(filepath.Join(checkoutPath, "snapcraft.yaml"))
	c.Assert(err, IsNil)
	a := createTestSnapcraftArtifact(checkoutPath)

	ins := craft.NewSnapcraftInspector(getTestSnapcraftConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.snapcraft"]
	c.Assert(inspection.Opinion, Equals, tc.opinion)
	c.Assert(inspection.Reason, Equals, tc.reason)
}

func (s *snapcraftSuite) TestSnapcraftGitInspectArtifactUnableToDecodeSnapcraftYaml(c *C) {
	tc := struct {
		opinion opinions.OpinionKind
		reason  string
	}{
		opinions.Rejected,
		"cannot decode snapcraft.yaml",
	}
	f, err := loadTestSnapcraftArtifactData()
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	restorer := craft.MockOsOpen(func(string) (*os.File, error) {
		temp, err := os.CreateTemp("", "snapcraft-empty.yaml")
		if err != nil {
			return nil, err
		}
		defer func() { _ = temp.Close() }()
		defer func() { _ = os.Remove(temp.Name()) }()
		return os.Open(temp.Name())
	})
	defer restorer()

	checkoutPath := c.MkDir()
	_, err = os.Create(filepath.Join(checkoutPath, "snapcraft.yaml"))
	c.Assert(err, IsNil)
	a := createTestSnapcraftArtifact(checkoutPath)

	ins := craft.NewSnapcraftInspector(getTestSnapcraftConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.snapcraft"]
	c.Assert(inspection.Opinion, Equals, tc.opinion)
	c.Assert(inspection.Reason, Equals, tc.reason)
}

func (s *snapcraftSuite) TestGetSnapcraftYaml(c *C) {
	for _, tc := range []struct {
		path        string
		shouldFind bool
	}{
		{"snap/snapcraft.yaml", true},
		{"snapcraft.yaml", true},
		{"build-aux/snap/snapcraft.yaml", true},
		{"build-aux/snapcraft.yaml", false},
		{"fakecraft.yaml", false},
	} {
		dir, err := os.MkdirTemp("", "TestGetSnapcraftYaml")
		if err != nil {
			c.Fatal(err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		full := filepath.Join(dir, tc.path)
		dirs, _ := filepath.Split(full)
		if err = os.MkdirAll(dirs, 0755); err != nil {
			c.Fatal(err)
		}

		fo, err := os.Create(full)
		if err != nil {
			c.Fatal(err)
		}
		defer func() { _ = fo.Close() }()
		if _, err := fo.WriteString("name: my-project"); err != nil {
			c.Fatal(err)
		}

		snapcraftPath, found := craft.GetSnapcraftYamlPath(dir)

		c.Assert(found, Equals, tc.shouldFind)
		if found {
			c.Assert(snapcraftPath, Equals, full)
		}
	}

}
