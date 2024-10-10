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
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/gabriel-vasile/mimetype"
)

type rockcraftSuite struct{}

var _ = Suite(&rockcraftSuite{})

func (t *rockcraftSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

func RockcraftTest(t *testing.T) { TestingT(t) }

func getTestRockcraftConfig() config.CraftsInspectorConfig {
	return config.CraftsInspectorConfig{
		Urls: []glob.Glob{
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

func createTestRockcraftArtefact(is_shallow bool) *metadata.Artefact {
	a := metadata.NewArtefact()
	a.Request, _ = http.NewRequest("GET", "https://example.com:443/test/git-upload-pack", nil)
	a.CurrentDownload.ContentType = "application/x-git-upload-pack-result"
	a.Request.Body = io.NopCloser(strings.NewReader("0014command=fetch\n0000"))
	a.MimeType = mimetype.Lookup("application/octet-stream")
	a.RequestInspection = metadata.InspectionMap{
		"git.upload-pack": &Inspection{
			Opinion: opinions.Pending,
			Reason:  "valid URL for rockcraft upload-pack",
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
				"is-shallow": is_shallow,
			},
		},
	}
	return a
}

func loadTestRockcraftArtefactData() (*files.ArtefactFile, error) {
	git_capture := filepath.Join("testdata", "rockcraftpkg.raw")
	file, err := files.OpenArtefactFile(git_capture)
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
		a := metadata.NewArtefact()
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

func (s *rockcraftSuite) TestRockcraftGitInspectArtefact(c *C) {
	for _, tc := range []struct {
		is_shallow bool
		opinion    opinions.OpinionKind
		reason     string
	}{
		{true, opinions.Approved, "rockcraft repository found"},
		{false, opinions.Rejected, "rockcraft repository is not shallow"},
	} {

		a := createTestRockcraftArtefact(tc.is_shallow)
		f, err := loadTestRockcraftArtefactData()
		c.Assert(err, IsNil)
		defer f.Close()

		ins := craft.NewRockcraftInspector(getTestRockcraftConfig())
		err = ins.InspectArtefact(f, a)
		c.Assert(err, IsNil)

		inspection := a.ResponseInspection["craft.rockcraft"]
		c.Assert(inspection.Opinion, Equals, tc.opinion)
		c.Assert(inspection.Reason, Equals, tc.reason)

		if tc.opinion == opinions.Approved {
			c.Check(a.Metadata.Type, Equals, "application/x.canonical.rockcraft")
			c.Check(a.Metadata.Name, Equals, "charmcraft-core22")
			c.Check(a.Metadata.Version, Equals, "3.1.2")
			c.Check(a.Metadata.Description, Equals, "Pack Ubuntu 22.04 charms")
			// FIXME: add more fields to test data
		}
	}
}

func (s *rockcraftSuite) TestRockcraftGitInspectArtefactMissingRockcraftYaml(c *C) {
	tc := struct {
		is_shallow bool
		opinion    opinions.OpinionKind
		reason     string
	}{
		true,
		opinions.Unknown,
		"git repository does not contain a rockcraft.yaml file",
	}
	restorer := craft.MockOsStat(func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	})
	defer restorer()

	a := createTestRockcraftArtefact(tc.is_shallow)
	f, err := loadTestRockcraftArtefactData()
	c.Assert(err, IsNil)
	defer f.Close()

	ins := craft.NewRockcraftInspector(getTestRockcraftConfig())

	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.rockcraft"]
	c.Assert(inspection.Opinion, Equals, tc.opinion)
	c.Assert(inspection.Reason, Equals, tc.reason)
}

func (s *rockcraftSuite) TestRockcraftGitInspectArtefactUnreadableRockcraftYaml(c *C) {
	tc := struct {
		is_shallow bool
		opinion    opinions.OpinionKind
		reason     string
	}{
		true,
		opinions.Rejected,
		"cannot open rockcraft.yaml file",
	}

	restorer := craft.MockOsOpen(func(string) (*os.File, error) {
		return nil, os.ErrNotExist
	})
	defer restorer()
	f, err := loadTestRockcraftArtefactData()
	c.Assert(err, IsNil)
	defer f.Close()

	a := createTestRockcraftArtefact(tc.is_shallow)

	ins := craft.NewRockcraftInspector(getTestRockcraftConfig())
	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.rockcraft"]
	c.Assert(inspection.Opinion, Equals, tc.opinion)
	c.Assert(inspection.Reason, Equals, tc.reason)
}

func (s *rockcraftSuite) TestRockcraftGitInspectArtefactUnableToDecodeRockcraftYaml(c *C) {
	tc := struct {
		is_shallow bool
		opinion    opinions.OpinionKind
		reason     string
	}{
		true,
		opinions.Rejected,
		"cannot decode rockcraft.yaml",
	}
	f, err := loadTestRockcraftArtefactData()
	c.Assert(err, IsNil)
	defer f.Close()

	restorer := craft.MockOsOpen(func(string) (*os.File, error) {
		temp, _ := os.CreateTemp("", "rockcraft-empty.yaml")
		defer temp.Close()
		defer os.Remove(temp.Name())
		return os.Open(temp.Name())
	})
	defer restorer()

	a := createTestRockcraftArtefact(tc.is_shallow)

	ins := craft.NewRockcraftInspector(getTestRockcraftConfig())
	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)

	inspection := a.ResponseInspection["craft.rockcraft"]
	c.Assert(inspection.Opinion, Equals, tc.opinion)
	c.Assert(inspection.Reason, Equals, tc.reason)
}
