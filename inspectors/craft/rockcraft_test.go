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
)

type rockcraftSuite struct {
	slog logger.Logger
}

var _ = Suite(&rockcraftSuite{logger.NewSessionLogger("test")})

func (t *rockcraftSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

func RockcraftTest(t *testing.T) { TestingT(t) }

func getTestRockcraftConfig() config.CraftsInspectorConfig {
	return config.CraftsInspectorConfig{
		URLs: []glob.Glob{
			glob.MustCompile("https://github.com:443/**"),
			glob.MustCompile("https://git.launchpad.net:443/**"),
		},
	}
}

func (s *rockcraftSuite) TestRockcraftInspectorInterface(c *C) {
	var iface Inspector
	ins := craft.NewRockcraftInspector(getTestRockcraftConfig())
	c.Assert(ins, Implements, &iface)

}

func (s *rockcraftSuite) TestUploadPackInspectorID(c *C) {
	ins := craft.NewRockcraftInspector(getTestRockcraftConfig())
	c.Assert(ins.ID(), Equals, "craft.rockcraft")

}

func loadTestRockcraftArtifactData() (*files.ArtifactFile, error) {
	gitCapture := filepath.Join("testdata", "rockcraftpkg.raw")
	file, err := files.OpenArtifactFile(gitCapture)
	return file, err
}

func (s *rockcraftSuite) TestInspectRockcraftGitRequest(c *C) {
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
		ins := craft.NewRockcraftInspector(getTestRockcraftConfig())
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

func (s *rockcraftSuite) TestRockcraftGitInspectArtifact(c *C) {
	for _, tc := range []struct {
		opinion opinions.OpinionKind
		reason  string
	}{
		{opinions.Approved, "rockcraft repository found"},
	} {
		f, err := loadTestRockcraftArtifactData()
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		checkoutPath := c.MkDir()
		err = git.UnpackObjects(f, checkoutPath, s.slog)
		c.Assert(err, IsNil)
		err = git.Checkout(checkoutPath, "d9c2c0282d81a993c0011113996b541a1ef1ebc7", s.slog)
		c.Assert(err, IsNil)

		a := createTestCraftArtifact(checkoutPath)

		f, err = loadTestRockcraftArtifactData()
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		ins := craft.NewRockcraftInspector(getTestRockcraftConfig())
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		inspection := a.ResponseInspection["craft.rockcraft"]
		c.Assert(inspection.Opinion, Equals, tc.opinion)
		c.Assert(inspection.Reason, Equals, tc.reason)

		if tc.opinion == opinions.Approved {
			c.Check(a.Metadata.Type, Equals, "application/x.canonical.rockcraft")
			c.Check(a.Metadata.Name, Equals, "charmcraft-core22")
			c.Check(a.Metadata.Version, Equals, "3.1.2")
			c.Check(a.Metadata.Description, Equals, "Pack Ubuntu 22.04 charms")
			c.Check(a.Metadata.ContentID, Equals, "d9c2c0282d81a993c0011113996b541a1ef1ebc7")
		}
	}
}

func (s *rockcraftSuite) TestRockcraftGitInspectArtifactMissingRockcraftYaml(c *C) {
	a := createTestCraftArtifact(c.MkDir())
	f, err := loadTestRockcraftArtifactData()
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	ins := craft.NewRockcraftInspector(getTestRockcraftConfig())

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	_, ok := a.ResponseInspection["craft.rockcraft"]
	c.Assert(ok, Equals, false)
}

func (s *rockcraftSuite) TestRockcraftGitInspectArtifactUnreadableRockcraftYaml(c *C) {
	tc := struct {
		opinion opinions.OpinionKind
		reason  string
	}{
		opinions.Rejected,
		"cannot open rockcraft.yaml file",
	}

	f, err := loadTestRockcraftArtifactData()
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	restorer := craft.MockOsOpen(func(string) (*os.File, error) {
		return nil, os.ErrNotExist
	})
	defer restorer()

	checkoutPath := c.MkDir()
	_, err = os.Create(filepath.Join(checkoutPath, "rockcraft.yaml"))
	c.Assert(err, IsNil)
	a := createTestSnapcraftArtifact(checkoutPath)

	ins := craft.NewRockcraftInspector(getTestRockcraftConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.rockcraft"]
	c.Assert(inspection.Opinion, Equals, tc.opinion)
	c.Assert(inspection.Reason, Equals, tc.reason)
}

func (s *rockcraftSuite) TestRockcraftGitInspectArtifactUnableToDecodeRockcraftYaml(c *C) {
	tc := struct {
		opinion opinions.OpinionKind
		reason  string
	}{
		opinions.Rejected,
		"cannot decode rockcraft.yaml",
	}
	f, err := loadTestRockcraftArtifactData()
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	restorer := craft.MockOsOpen(func(string) (*os.File, error) {
		temp, err := os.CreateTemp("", "rockcraft-empty.yaml")
		if err != nil {
			return nil, err
		}
		defer func() { _ = temp.Close() }()
		defer func() { _ = os.Remove(temp.Name()) }()
		return os.Open(temp.Name())
	})
	defer restorer()

	checkoutPath := c.MkDir()
	_, err = os.Create(filepath.Join(checkoutPath, "rockcraft.yaml"))
	c.Assert(err, IsNil)

	a := createTestCraftArtifact(checkoutPath)

	ins := craft.NewRockcraftInspector(getTestRockcraftConfig())
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.rockcraft"]
	c.Assert(inspection.Opinion, Equals, tc.opinion)
	c.Assert(inspection.Reason, Equals, tc.reason)
}
