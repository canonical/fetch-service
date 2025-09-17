// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2025 Canonical Ltd.
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
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/gabriel-vasile/mimetype"
)

type DummyReader struct{}

func (d DummyReader) Len() int {
	panic("Unexpected call to Len()")
}

func (d DummyReader) Read(p []byte) (n int, err error) {
	panic("Unexpected call to Read()")
}

func (d DummyReader) Seek(offset int64, whence int) (int64, error) {
	panic("Unexpected call to Seek()")

}
func (d DummyReader) ReadAt(p []byte, off int64) (n int, err error) {
	panic("Unexpected call to ReadAt()")

}

type charmcraftSuite struct {
	slog logger.Logger
}

var _ = Suite(&charmcraftSuite{logger.NewSessionLogger("test")})

func (t *charmcraftSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

func CharmcraftTest(t *testing.T) { TestingT(t) }

func getTestCharmcraftConfig() config.CraftsInspectorConfig {
	return config.CraftsInspectorConfig{
		Urls: []glob.Glob{
			glob.MustCompile("https://github.com:443/**"),
			glob.MustCompile("https://git.launchpad.net:443/**"),
		},
	}
}

func (s *charmcraftSuite) TestCharmcraftInspectorInterface(c *C) {
	var iface Inspector
	ins := craft.NewCharmcraftInspector(getTestCharmcraftConfig())
	c.Assert(ins, Implements, &iface)

}

func (s *charmcraftSuite) TestUploadPackInspectorID(c *C) {
	ins := craft.NewCharmcraftInspector(getTestCharmcraftConfig())
	c.Assert(ins.ID(), Equals, "craft.charmcraft")

}

func createTestCharmcraftArtifact(checkoutPath string) *metadata.Artifact {
	a := metadata.NewArtifact()
	a.Request, _ = http.NewRequest("GET", "https://example.com:443/test/git-upload-pack", nil)
	a.CurrentDownload.ContentType = "application/x-git-upload-pack-result"
	a.Request.Body = io.NopCloser(strings.NewReader("0014command=fetch\n0000"))
	a.MimeType = mimetype.Lookup("application/octet-stream")
	a.RequestInspection = metadata.InspectionMap{
		"git.upload-pack": &Inspection{
			Opinion: opinions.Pending,
			Reason:  "valid URL for charmcraft upload-pack",
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

type charmcraftInspectRequestTest struct {
	url      string
	approved bool
}

var charmcraftInspectRequestTests = []charmcraftInspectRequestTest{
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
}

func (s *charmcraftSuite) TestInspectCharmcraftGitRequest(c *C) {
	for _, tc := range charmcraftInspectRequestTests {
		ins := craft.NewCharmcraftInspector(getTestCharmcraftConfig())
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

func (s *charmcraftSuite) TestCharmcraftGitInspectArtifact(c *C) {
	checkoutPath := filepath.Join("testdata", "charmcraft-checkout")
	a := createTestCraftArtifact(checkoutPath)

	ins := craft.NewCharmcraftInspector(getTestCharmcraftConfig())

	err := ins.InspectArtifact(DummyReader{}, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.charmcraft"]
	c.Assert(inspection.Opinion, Equals, opinions.Approved)
	c.Assert(inspection.Reason, Equals, "charmcraft repository found")

	c.Check(a.Metadata.Type, Equals, "application/x.canonical.charmcraft")
	c.Check(a.Metadata.Name, Equals, "sample-charmcraft-project")
	c.Check(a.Metadata.Version, Equals, "")
	c.Check(a.Metadata.Description, Equals, "A very short one-line summary of the charm.")
}

func (s *charmcraftSuite) TestCharmcraftGitInspectArtifactMissingCharmcraftYaml(c *C) {
	restorer := craft.MockOsStat(func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	})
	defer restorer()

	a := createTestCharmcraftArtifact(c.MkDir())

	ins := craft.NewCharmcraftInspector(getTestCharmcraftConfig())

	err := ins.InspectArtifact(DummyReader{}, a)
	c.Assert(err, IsNil)

	_, ok := a.ResponseInspection["craft.charmcraft"]
	c.Assert(ok, Equals, false)
}

func (s *charmcraftSuite) TestCharmcraftGitInspectArtifactUnreadableCharmcraftYaml(c *C) {
	restorer := craft.MockOsOpen(func(string) (*os.File, error) {
		return nil, os.ErrNotExist
	})
	defer restorer()

	checkoutPath := c.MkDir()
	_, err := os.Create(filepath.Join(checkoutPath, "charmcraft.yaml"))
	c.Assert(err, IsNil)
	a := createTestSnapcraftArtifact(checkoutPath)

	ins := craft.NewCharmcraftInspector(getTestCharmcraftConfig())
	err = ins.InspectArtifact(DummyReader{}, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.charmcraft"]
	c.Assert(inspection.Opinion, Equals, opinions.Rejected)
	c.Assert(inspection.Reason, Equals, "cannot open charmcraft.yaml file")
}

func (s *charmcraftSuite) TestCharmcraftGitInspectArtifactUnableToDecodeCharmcraftYaml(c *C) {
	restorer := craft.MockOsOpen(func(string) (*os.File, error) {
		temp, _ := os.CreateTemp("", "charmcraft-empty.yaml")
		defer temp.Close()
		defer os.Remove(temp.Name())
		return os.Open(temp.Name())
	})
	defer restorer()

	checkoutPath := c.MkDir()
	_, err := os.Create(filepath.Join(checkoutPath, "charmcraft.yaml"))
	c.Assert(err, IsNil)

	a := createTestCraftArtifact(checkoutPath)

	ins := craft.NewCharmcraftInspector(getTestCharmcraftConfig())
	err = ins.InspectArtifact(DummyReader{}, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.charmcraft"]
	c.Assert(inspection.Opinion, Equals, opinions.Rejected)
	c.Assert(inspection.Reason, Equals, "cannot decode charmcraft.yaml")
}

func (s *charmcraftSuite) TestCharmcraftGitInspectArtifactNoGitCheckout(c *C) {
	a := createTestCraftArtifact("")

	ins := craft.NewCharmcraftInspector(getTestCharmcraftConfig())
	err := ins.InspectArtifact(DummyReader{}, a)
	c.Assert(err, IsNil)

	_, ok := a.ResponseInspection["craft.charmcraft"]
	c.Assert(ok, Equals, false)
}
