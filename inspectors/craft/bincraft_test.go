// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2026 Canonical Ltd.
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

type bincraftSuite struct {
	sl logger.Logger
}

var _ = Suite(&bincraftSuite{logger.NewSessionLogger("test")})

func (t *bincraftSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

func BincraftTest(t *testing.T) { TestingT(t) }

func getTestBincraftConfig() config.CraftsInspectorConfig {
	return config.CraftsInspectorConfig{
		URLs: []glob.Glob{
			glob.MustCompile("https://github.com:443/**"),
			glob.MustCompile("https://git.launchpad.net:443/**"),
		},
	}
}

func (s *bincraftSuite) TestBincraftInspectorInterface(c *C) {
	var iface Inspector
	ins := craft.NewBincraftInspector(getTestBincraftConfig())
	c.Assert(ins, Implements, &iface)

}

func (s *bincraftSuite) TestUploadPackInspectorID(c *C) {
	ins := craft.NewBincraftInspector(getTestBincraftConfig())
	c.Assert(ins.ID(), Equals, "craft.bincraft")

}

func createTestBincraftArtifact(checkoutPath string) *metadata.Artifact {
	a := metadata.NewArtifact()
	a.Request, _ = http.NewRequest("GET", "https://example.com:443/test/git-upload-pack", nil)
	a.CurrentDownload.ContentType = "application/x-git-upload-pack-result"
	a.Request.Body = io.NopCloser(strings.NewReader("0014command=fetch\n0000"))
	a.MimeType = mimetype.Lookup("application/octet-stream")
	a.RequestInspection = metadata.InspectionMap{
		"git.upload-pack": &Inspection{
			Opinion: opinions.Pending,
			Reason:  "valid URL for bincraft upload-pack",
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
					"want fce2c3b18235c1383fd76958d4ee3d20f5865bfe",
					"done",
				},
				"repository": "https://my.repo/foo",
				"command":    "fetch",
				"project":    "bump2version",
				"protocol":   "version=2",
				"wants": []string{
					"fce2c3b18235c1383fd76958d4ee3d20f5865bfe",
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

func loadTestBincraftArtifactData() (*files.ArtifactFile, error) {
	bincraftpkgFile := filepath.Join("testdata", "bincraftpkg.raw")
	file, err := files.OpenArtifactFile(bincraftpkgFile)
	return file, err
}

func (s *bincraftSuite) TestInspectBincraftGitRequest(c *C) {
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
		ins := craft.NewBincraftInspector(getTestBincraftConfig())
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
		c.Assert(ok, Equals, tc.approved, Commentf("Approval status is wrong for '%s' (%t != %t)", tc.url, ok, tc.approved))
		if tc.approved {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		}
	}
}

func (s *bincraftSuite) TestBincraftGitInspectArtifact(c *C) {
	for _, tc := range []struct {
		opinion opinions.OpinionKind
		reason  string
	}{
		{opinions.Approved, "bincraft repository found"},
	} {

		f, err := loadTestBincraftArtifactData()
		c.Assert(err, IsNil)
		defer f.Close()

		checkoutPath := c.MkDir()
		err = git.UnpackObjects(f, checkoutPath, s.sl)
		c.Assert(err, IsNil)
		err = git.Checkout(checkoutPath, "fce2c3b18235c1383fd76958d4ee3d20f5865bfe", s.sl)
		c.Assert(err, IsNil)

		a := createTestBincraftArtifact(checkoutPath)

		f, err = loadTestBincraftArtifactData()
		c.Assert(err, IsNil)
		defer f.Close()

		ins := craft.NewBincraftInspector(getTestBincraftConfig())
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		inspection := a.ResponseInspection["craft.bincraft"]
		c.Assert(inspection.Opinion, Equals, tc.opinion)
		c.Assert(inspection.Reason, Equals, tc.reason)

		if tc.opinion == opinions.Approved {
			c.Check(a.Metadata.Type, Equals, "application/x.canonical.bincraft")
			c.Check(a.Metadata.Name, Equals, "temp")
			c.Check(a.Metadata.Version, Equals, "0.1")
			c.Check(a.Metadata.Description, Equals, "A very short one-line summary of the package.")
			c.Check(a.Metadata.ContentID, Equals, "fce2c3b18235c1383fd76958d4ee3d20f5865bfe")
		}
	}
}

func (s *bincraftSuite) TestBincraftGitInspectArtifactMissingBincraftYaml(c *C) {
	restorer := craft.MockOsStat(func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	})
	defer restorer()

	a := createTestBincraftArtifact(c.MkDir())
	f, err := loadTestBincraftArtifactData()
	c.Assert(err, IsNil)
	defer f.Close()

	ins := craft.NewBincraftInspector(getTestBincraftConfig())

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	_, ok := a.ResponseInspection["craft.bincraft"]
	c.Assert(ok, Equals, false)
}

func (s *bincraftSuite) TestBincraftGitInspectArtifactUnreadableBincraftYaml(c *C) {
	tc := struct {
		opinion opinions.OpinionKind
		reason  string
	}{
		opinions.Rejected,
		"cannot open bincraft.yaml file",
	}

	f, err := loadTestBincraftArtifactData()
	c.Assert(err, IsNil)
	defer f.Close()

	restorer := craft.MockOsOpen(func(string) (*os.File, error) {
		return nil, os.ErrNotExist
	})
	defer restorer()

	checkoutPath := c.MkDir()
	_, err = os.Create(filepath.Join(checkoutPath, "bincraft.yaml"))
	c.Assert(err, IsNil)
	a := createTestBincraftArtifact(checkoutPath)

	ins := craft.NewBincraftInspector(getTestBincraftConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.bincraft"]
	c.Assert(inspection.Opinion, Equals, tc.opinion)
	c.Assert(inspection.Reason, Equals, tc.reason)
}

func (s *bincraftSuite) TestBincraftGitInspectArtifactUnableToDecodeBincraftYaml(c *C) {
	tc := struct {
		opinion opinions.OpinionKind
		reason  string
	}{
		opinions.Rejected,
		"cannot decode bincraft.yaml",
	}
	f, err := loadTestBincraftArtifactData()
	c.Assert(err, IsNil)
	defer f.Close()

	restorer := craft.MockOsOpen(func(string) (*os.File, error) {
		temp, _ := os.CreateTemp("", "bincraft-empty.yaml")
		defer temp.Close()
		defer os.Remove(temp.Name())
		return os.Open(temp.Name())
	})
	defer restorer()

	checkoutPath := c.MkDir()
	_, err = os.Create(filepath.Join(checkoutPath, "bincraft.yaml"))
	c.Assert(err, IsNil)

	a := createTestBincraftArtifact(checkoutPath)

	ins := craft.NewBincraftInspector(getTestBincraftConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.bincraft"]
	c.Assert(inspection.Opinion, Equals, tc.opinion)
	c.Assert(inspection.Reason, Equals, tc.reason)
}
