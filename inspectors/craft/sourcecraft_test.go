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

func createTestCraftArtifact(checkoutPath string) *metadata.Artifact {
	a := metadata.NewArtifact()
	a.Request, _ = http.NewRequest("GET", "https://example.com:443/test/git-upload-pack", nil)
	a.CurrentDownload.ContentType = "application/x-git-upload-pack-result"
	a.Request.Body = io.NopCloser(strings.NewReader("0014command=fetch\n0000"))
	a.MimeType = mimetype.Lookup("application/octet-stream")
	a.RequestInspection = metadata.InspectionMap{
		"git.upload-pack": &Inspection{
			Opinion: opinions.Pending,
			Reason:  "valid URL for craft upload-pack",
			Annotations: Annotation{
				"client-request": []string{
					"command=fetch",
					"agent=git/2.45.2",
					"object-format=sha1",
					"",
					"thin-pack",
					"no-progress",
					"include-tag",
					"ofs-delta",
					"deepen 1",
					"want d9c2c0282d81a993c0011113996b541a1ef1ebc7",
					"done",
				},
				"repository": "https://github.com:443/lengau/charmcraft-rocks",
				"command":    "fetch",
				"project":    "charmcraft-core22",
				"protocol":   "version=2",
				"wants": []string{
					"d9c2c0282d81a993c0011113996b541a1ef1ebc7",
				},
				"is-shallow": true,
			},
		},
	}

	annot := Annotation{}
	if len(checkoutPath) > 0 {
		annot["git-checkout-path"] = checkoutPath
	}
	a.ResponseInspection = metadata.InspectionMap{
		"git.upload-pack": &Inspection{
			Opinion:     opinions.Unknown,
			Reason:      "",
			Annotations: annot,
		},
	}
	return a
}

type sourcecraftSuite struct {
	slog logger.Logger
}

var _ = Suite(&sourcecraftSuite{logger.NewSessionLogger("test")})

func (t *sourcecraftSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

func Test(t *testing.T) { TestingT(t) }

func getTestSourcecraftConfig() config.CraftsInspectorConfig {
	return config.CraftsInspectorConfig{
		URLs: []glob.Glob{
			glob.MustCompile("https://github.com:443/**"),
			glob.MustCompile("https://git.launchpad.net:443/**"),
		},
	}
}

func (s *sourcecraftSuite) TestSourcecraftInspectorInterface(c *C) {
	var iface Inspector
	ins := craft.NewSourcecraftInspector(getTestSourcecraftConfig())
	c.Assert(ins, Implements, &iface)

}

func (s *sourcecraftSuite) TestUploadPackInspectorID(c *C) {
	ins := craft.NewSourcecraftInspector(getTestSourcecraftConfig())
	c.Assert(ins.ID(), Equals, "craft.sourcecraft")

}

func createTestSourcecraftArtifact(checkoutPath string) *metadata.Artifact {
	a := metadata.NewArtifact()
	a.Request, _ = http.NewRequest("GET", "https://example.com:443/test/git-upload-pack", nil)
	a.CurrentDownload.ContentType = "application/x-git-upload-pack-result"
	a.Request.Body = io.NopCloser(strings.NewReader("0014command=fetch\n0000"))
	a.MimeType = mimetype.Lookup("application/octet-stream")
	a.RequestInspection = metadata.InspectionMap{
		"git.upload-pack": &Inspection{
			Opinion: opinions.Pending,
			Reason:  "valid URL for sourcecraft upload-pack",
			Annotations: Annotation{
				"client-request": []string{
					"command=fetch",
					"agent=git/2.34.1",
					"object-format=sha1",
					"",
					"thin-pack",
					"no-progress",
					"include-tag",
					"ofs-delta",
					"deepen 1",
					"want 10fce2c8e3a341998ffd2aa4e27b02699d1bb5ad",
					"done",
				},
				"repository": "https://my.repo/foo",
				"command":    "fetch",
				"project":    "bump2version",
				"protocol":   "version=2",
				"wants": []string{
					"10fce2c8e3a341998ffd2aa4e27b02699d1bb5ad",
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

func loadTestSourcecraftArtifactData() (*files.ArtifactFile, error) {
	sourcepkgFile := filepath.Join("testdata", "sourcepkg.raw")
	file, err := files.OpenArtifactFile(sourcepkgFile)
	return file, err
}

func (s *sourcecraftSuite) TestInspectSourcecraftGitRequest(c *C) {
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
		ins := craft.NewSourcecraftInspector(getTestSourcecraftConfig())
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

func (s *sourcecraftSuite) TestSourcecraftGitInspectArtifact(c *C) {
	for _, tc := range []struct {
		opinion opinions.OpinionKind
		reason  string
	}{
		{opinions.Approved, "sourcecraft repository found"},
	} {

		f, err := loadTestSourcecraftArtifactData()
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		checkoutPath := c.MkDir()
		err = git.UnpackObjects(f, checkoutPath, s.slog)
		c.Assert(err, IsNil)
		err = git.Checkout(checkoutPath, "10fce2c8e3a341998ffd2aa4e27b02699d1bb5ad", s.slog)
		c.Assert(err, IsNil)

		a := createTestCraftArtifact(checkoutPath)

		f, err = loadTestSourcecraftArtifactData()
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		ins := craft.NewSourcecraftInspector(getTestSourcecraftConfig())
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		inspection := a.ResponseInspection["craft.sourcecraft"]
		c.Assert(inspection.Opinion, Equals, tc.opinion)
		c.Assert(inspection.Reason, Equals, tc.reason)

		if tc.opinion == opinions.Approved {
			c.Check(a.Metadata.Type, Equals, "application/x.canonical.sourcecraft")
			c.Check(a.Metadata.Name, Equals, "autossh")
			c.Check(a.Metadata.Version, Equals, "git")
			c.Check(a.Metadata.Description, Equals, "A very short one-line summary of the package.")
			c.Check(a.Metadata.ContentID, Equals, "d9c2c0282d81a993c0011113996b541a1ef1ebc7")
		}
	}
}

func (s *sourcecraftSuite) TestSourcecraftGitInspectArtifactMissingSourcecraftYaml(c *C) {
	restorer := craft.MockOsStat(func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	})
	defer restorer()

	a := createTestSourcecraftArtifact(c.MkDir())
	f, err := loadTestSourcecraftArtifactData()
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	ins := craft.NewSourcecraftInspector(getTestSourcecraftConfig())

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	_, ok := a.ResponseInspection["craft.sourcecraft"]
	c.Assert(ok, Equals, false)
}

func (s *sourcecraftSuite) TestSourcecraftGitInspectArtifactUnreadableSourcecraftYaml(c *C) {
	tc := struct {
		opinion opinions.OpinionKind
		reason  string
	}{
		opinions.Rejected,
		"cannot open sourcecraft.yaml file",
	}

	f, err := loadTestSourcecraftArtifactData()
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	restorer := craft.MockOsOpen(func(string) (*os.File, error) {
		return nil, os.ErrNotExist
	})
	defer restorer()

	checkoutPath := c.MkDir()
	_, err = os.Create(filepath.Join(checkoutPath, "sourcecraft.yaml"))
	c.Assert(err, IsNil)
	a := createTestSnapcraftArtifact(checkoutPath)

	ins := craft.NewSourcecraftInspector(getTestSourcecraftConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.sourcecraft"]
	c.Assert(inspection.Opinion, Equals, tc.opinion)
	c.Assert(inspection.Reason, Equals, tc.reason)
}

func (s *sourcecraftSuite) TestSourcecraftGitInspectArtifactUnableToDecodeSourcecraftYaml(c *C) {
	tc := struct {
		opinion opinions.OpinionKind
		reason  string
	}{
		opinions.Rejected,
		"cannot decode sourcecraft.yaml",
	}
	f, err := loadTestSourcecraftArtifactData()
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	restorer := craft.MockOsOpen(func(string) (*os.File, error) {
		temp, err := os.CreateTemp("", "sourcecraft-empty.yaml")
		if err != nil {
			return nil, err
		}
		_ = temp.Close()
		return os.Open(temp.Name())
	})
	defer restorer()

	checkoutPath := c.MkDir()
	_, err = os.Create(filepath.Join(checkoutPath, "sourcecraft.yaml"))
	c.Assert(err, IsNil)

	a := createTestCraftArtifact(checkoutPath)

	ins := craft.NewSourcecraftInspector(getTestSourcecraftConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.sourcecraft"]
	c.Assert(inspection.Opinion, Equals, tc.opinion)
	c.Assert(inspection.Reason, Equals, tc.reason)
}
