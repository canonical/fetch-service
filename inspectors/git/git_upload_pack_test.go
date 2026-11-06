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

package git_test

import (
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/git"
	"github.com/canonical/fetch-service/inspectors/git/config"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type uploadPackSuite struct{}

var _ = Suite(&uploadPackSuite{})

func (t *uploadPackSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

func (s *uploadPackSuite) TestUploadPackInspectorInterface(c *C) {
	var iface Inspector
	ins := git.NewUploadPackInspector(config.GitInspectorConfig{})
	c.Assert(ins, Implements, &iface)

}

func (s *uploadPackSuite) TestUploadPackInspectorID(c *C) {
	ins := git.NewUploadPackInspector(config.GitInspectorConfig{})
	c.Assert(ins.ID(), Equals, "git.upload-pack")

}

func (s *uploadPackSuite) TestInspectLsRefsRequest(c *C) {
	for _, tc := range []struct {
		url      string
		approved bool
	}{
		{"https://github.com:443/user/project.git/git-upload-pack", true},
		{"https://git.launchpad.net:443/project/git-upload-pack", true},
		{"https://git.launchpad.net:443/~user/project/+git/project/git-upload-pack", true},
		{"https://invalid.com:443/user/project.git/git-upload-pack", false},
		{"http://github.com/user/project.git/git-upload-pack", false},
		{"https://gothub.com:443/user/project.git/git-upload-pack", false},
		{"ahttps://github.com:443/user/project.git/git-upload-pack", false},
		{"https://github.com:443/user/project.git/git-upload-packs", false},
		{"https://github.com:443/user/project.git/something-else", false},
		{"https://git.launchpad.com:443/project/git-upload-pack", false},
		{"https://git.lpad.net:443/~user/project/+git/project/git-upload-pack", false},
	} {
		ins := git.NewUploadPackInspector(getTestConfig())
		a := fakeGitArtifact()
		a.CurrentDownload.URL = tc.url
		var err error
		a.Request, err = http.NewRequest("GET", tc.url, nil)
		c.Assert(err, IsNil)
		a.Request.Body = io.NopCloser(strings.NewReader("0014command=ls-refs\n0000"))

		err = ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp := a.RequestInspection[ins.ID()]
		if tc.approved {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		}
	}
}

var uploadPackLsRefsArtifactData = `00526b99254b1c5c823d054bc0ae1ebccfa070380fce HEAD symref-target:refs/heads/master
004497cea5a48e9144f83ff4d7211c6d1c38bc42d014 refs/heads/fix/release
003f6b99254b1c5c823d054bc0ae1ebccfa070380fce refs/heads/master
003e8cb123237fbe2b13a1108fa7b876c9ad0c12a3b1 refs/tags/v0.5.0
003ea0dda31a428ae903914ac405ee3b81925b83985e refs/tags/v0.5.1
006f0bfe79093aaafeb51c6bf16e884c8acc3629deeb refs/tags/v0.5.10 peeled:8c6a8b587d0818eca0fd6cd70fea1451b7f3515e
006f1f7cf26e28c9544c31dc84924fe394a33bff7c0b refs/tags/v0.5.11 peeled:eef8eae16fc3c4a2ae1c82fc688a44b3fd023e1e
003e2c53e48f96633cf8427c1dcaf020d8610ece0aa6 refs/tags/v0.5.2
003e769b5c56b566d5300deb0ac05fb5564f31df1ee0 refs/tags/v0.5.3
003edbb42f3374e0dea543ff633f0e02059e29db833e refs/tags/v0.5.4
003e92fa4dda4000a131f5fb7b7c99dd9ccd47e5f4bc refs/tags/v0.5.5
006e0164a46b6f73296c783b92184ab171919b29f789 refs/tags/v0.5.6 peeled:74b720957a7be9bae956fdedf34f1dd34364e9a7
006e30689830e58e5e39a57c9095b35e4ac1d7623a25 refs/tags/v0.5.7 peeled:cc92f5b7e5851de2a68b10ba9982e8efe25824ac
006e69941b9d152f7b42289f5f5741ec040b6f0a2c05 refs/tags/v0.5.8 peeled:3c0053a0e6a56ef6b7cc65a92d9acec353494bd3
006e2eaa9fba5c97ca0810c49df39497f3f9be3ac7e6 refs/tags/v0.5.9 peeled:a702c1be7623a7510a0ec396b69923868c0dd027
006e1b4b4012d4c1f03a5d9ce4463b2687512d9b6d31 refs/tags/v1.0.0 peeled:70547660da19d0f768bc3310292873da9d81b3c0
006ee57a1f3a81f78545bfd7112472c38a2c7bf5485e refs/tags/v1.0.1 peeled:14fa60386d05b7971bcf2b76878da9e3c87760b5
0000`

func (s *uploadPackSuite) TestUploadPackInspectLsRefsArtifact(c *C) {
	for _, tc := range []struct {
		data   string
		errmsg string
	}{
		{uploadPackLsRefsArtifactData, ""},
		{uploadPackLsRefsArtifactData[1:], "decode error"},
		{
			uploadPackLsRefsArtifactData[:len(uploadPackLsRefsArtifactData)-1],
			`strconv.ParseUint: parsing "000\x00": invalid syntax`,
		},
	} {
		a := fakeGitArtifact()
		var err error
		a.Request, err = http.NewRequest("GET", "https://github.com:443/user/project.git/git-upload-pack", nil)
		c.Assert(err, IsNil)
		a.CurrentDownload.ContentType = "application/x-git-upload-pack-result"
		a.Request.Body = io.NopCloser(strings.NewReader("0014command=ls-refs\n0000"))
		a.RequestInspection = metadata.InspectionMap{
			"git.upload-pack": &Inspection{
				Opinion: opinions.Pending,
				Reason:  "valid URL for git upload-pack",
				Annotations: Annotation{
					"client-request": []string{
						"command=ls-refs",
						"agent=git/2.34.1",
						"object-format=sha1",
						"",
						"peel",
						"symrefs",
						"unborn",
						"ref-prefix HEAD",
						"ref-prefix refs/heads/",
						"ref-prefix master",
						"ref-prefix refs/master",
						"ref-prefix refs/tags/master",
						"ref-prefix refs/heads/master",
						"ref-prefix refs/remotes/master",
						"ref-prefix refs/remotes/master/HEAD",
						"ref-prefix refs/tags/",
					},
					"repository": "https://my.repo/foo",
					"command":    "ls-refs",
					"project":    "bump2version",
					"protocol":   "version=2",
				},
			},
		}

		f := strings.NewReader(tc.data)

		ins := git.NewUploadPackInspector(getTestConfig())
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		if tc.errmsg == "" {
			c.Assert(a.Approved(), Equals, true)
		} else {
			c.Assert(a.Rejected(), Equals, true)
			c.Assert(a.ResponseInspection[ins.ID()].Annotations["error-msg"], Equals, tc.errmsg, Commentf("test case: +%v"))
		}

		c.Check(a.Metadata.Type, Equals, "application/x.git.upload-pack-result.ls-ref")
		c.Assert(a.Approved(), Equals, tc.errmsg == "")
	}
}

func (s *uploadPackSuite) TestInspectFetchRequest(c *C) {
	url := "https://github.com:443/user/project.git/git-upload-pack"

	ins := git.NewUploadPackInspector(getTestConfig())
	a := fakeGitArtifact()
	a.CurrentDownload.URL = url
	a.Request, _ = http.NewRequest("GET", url, nil)
	a.Request.Body = io.NopCloser(strings.NewReader(
		"0012command=fetch\n" +
			"000ddeepen 1\n" +
			"0032want 6b99254b1c5c823d054bc0ae1ebccfa070380fce\n" +
			"0000",
	))

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	c.Assert(a.RequestInspection["git.upload-pack"], DeepEquals, &Inspection{
		Opinion: opinions.Pending,
		Reason:  "valid URL for git upload-pack",
		Annotations: Annotation{
			"repository": "https://github.com:443/user/project.git",
			"num-wants":  1,
			"wants":      []string{"6b99254b1c5c823d054bc0ae1ebccfa070380fce"},
			"is-shallow": true,
			"server":     "github.com",
			"project":    "project",
			"protocol":   "version=2",
			"command":    "fetch",
			"client-request": []string{
				"command=fetch",
				"deepen 1",
				"want 6b99254b1c5c823d054bc0ae1ebccfa070380fce",
			},
		},
	})
	c.Assert(a.RequestPending(), Equals, true)
	c.Assert(a.RequestRejected(), Equals, false)
}

func (s *uploadPackSuite) TestInspectFetchRequestUnsupportedProtocolVersions(c *C) {
	for _, tc := range []struct {
		a     *metadata.Artifact
		proto string
	}{
		{fakeGitArtifactUnsuportedProtocol(), "version=1"},
		{fakeGitArtifactNoProtocolVersion(), ""},
	} {
		url := "https://github.com:443/user/project.git/git-upload-pack"

		ins := git.NewUploadPackInspector(getTestConfig())
		a := tc.a
		a.CurrentDownload.URL = url
		a.Request, _ = http.NewRequest("GET", url, nil)
		a.Request.Body = io.NopCloser(strings.NewReader(
			"0012command=fetch\n" +
				"000ddeepen 1\n" +
				"0032want 6b99254b1c5c823d054bc0ae1ebccfa070380fce\n" +
				"0000",
		))

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)
		c.Assert(a.RequestInspection["git.upload-pack"], DeepEquals, &Inspection{
			Opinion:     opinions.Unknown,
			Reason:      "unsupported git protocol version",
			Annotations: Annotation{"proto": tc.proto},
		})
		c.Assert(a.RequestPending(), Equals, false)
		c.Assert(a.RequestRejected(), Equals, false)
	}
}

func (s *uploadPackSuite) TestInspectFetchRequestDuplicateRef(c *C) {
	url := "https://github.com:443/user/project.git/git-upload-pack"

	ins := git.NewUploadPackInspector(getTestConfig())
	a := fakeGitArtifact()
	a.CurrentDownload.URL = url
	a.Request, _ = http.NewRequest("GET", url, nil)
	a.Request.Body = io.NopCloser(strings.NewReader(
		"0012command=fetch\n" +
			"000ddeepen 1\n" +
			"0036want 6b99254b1c5c823d054bc0ae1ebccfa070380fce013f\n" +
			"0036want 6b99254b1c5c823d054bc0ae1ebccfa070380fce013f\n" +
			"0000",
	))

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	c.Assert(a.RequestInspection["git.upload-pack"], DeepEquals, &Inspection{
		Opinion: opinions.Pending,
		Reason:  "valid URL for git upload-pack",
		Annotations: Annotation{
			"repository": "https://github.com:443/user/project.git",
			"num-wants":  1,
			"wants":      []string{"6b99254b1c5c823d054bc0ae1ebccfa070380fce013f"},
			"is-shallow": true,
			"server":     "github.com",
			"project":    "project",
			"protocol":   "version=2",
			"command":    "fetch",
			"client-request": []string{
				"command=fetch",
				"deepen 1",
				"want 6b99254b1c5c823d054bc0ae1ebccfa070380fce013f",
				"want 6b99254b1c5c823d054bc0ae1ebccfa070380fce013f",
			},
		},
	})
	c.Assert(a.RequestPending(), Equals, true)
	c.Assert(a.RequestRejected(), Equals, false)
}

func (s *uploadPackSuite) TestInspectFetchRequestReject(c *C) {
	url := "https://github.com:443/user/project.git/git-upload-pack"

	ins := git.NewUploadPackInspector(getTestConfig())
	a := fakeGitArtifact()
	a.CurrentDownload.URL = url
	a.Request, _ = http.NewRequest("GET", url, nil)
	a.Request.Body = io.NopCloser(strings.NewReader(
		"0012command=fetch\n" +
			"0036want 6b99254b1c5c823d054bc0ae1ebccfa070380fce013f\n" +
			"0036want 006e69941b9d152f7b42289f5f5741ec040b6f0a2c05\n" +
			"0036want 006f0bfe79093aaafeb51c6bf16e884c8acc3629deeb\n" +
			"0000",
	))

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	c.Assert(a.RequestInspection["git.upload-pack"], DeepEquals, &Inspection{
		Opinion: opinions.Rejected,
		Reason:  "fetch is only allowed with depth 1",
		Annotations: Annotation{
			"num-wants": 3,
			"wants": []string{
				"6b99254b1c5c823d054bc0ae1ebccfa070380fce013f",
				"006e69941b9d152f7b42289f5f5741ec040b6f0a2c05",
				"006f0bfe79093aaafeb51c6bf16e884c8acc3629deeb",
			},
			"repository": "https://github.com:443/user/project.git",
			"is-shallow": false,
			"server":     "github.com",
			"project":    "project",
			"protocol":   "version=2",
			"command":    "fetch",
			"client-request": []string{
				"command=fetch",
				"want 6b99254b1c5c823d054bc0ae1ebccfa070380fce013f",
				"want 006e69941b9d152f7b42289f5f5741ec040b6f0a2c05",
				"want 006f0bfe79093aaafeb51c6bf16e884c8acc3629deeb",
			},
		},
	})
	c.Assert(a.RequestRejected(), Equals, true)
	c.Assert(a.RequestPending(), Equals, false)
}

func (s *uploadPackSuite) TestUploadPackInspectFetchArtifact(c *C) {
	for _, tc := range []struct {
		filename string
		errmsg   string
	}{
		{"testdata/sourcepkg.raw", ""},
		{"testdata/incorrect-git-object.raw", ""},
		{"testdata/bad-data.raw", `strconv.ParseUint: parsing "not-": invalid syntax`},
	} {
		a := fakeGitArtifact()
		a.Request, _ = http.NewRequest("GET", "https://github.com:443/user/project.git/git-upload-pack", nil)
		a.CurrentDownload.ContentType = "application/x-git-upload-pack-result"
		a.MimeType = mimetype.Lookup("application/octet-stream")
		a.Request.Body = io.NopCloser(strings.NewReader("0014command=fetch\n0000"))
		a.RequestInspection = metadata.InspectionMap{
			"git.upload-pack": &Inspection{
				Opinion: opinions.Pending,
				Reason:  "valid URL for git upload-pack",
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
					"num-wants":  1,
					"wants":      []string{"10fce2c8e3a341998ffd2aa4e27b02699d1bb5ad"},
				},
			},
		}

		a.SessionCacheDir = c.MkDir()

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)

		ins := git.NewUploadPackInspector(getTestConfig())
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		if tc.errmsg == "" {
			c.Assert(a.ResponseInspection[ins.ID()].Opinion, Equals, opinions.Unknown)

			// Check that the git repo was unpacked and checked-out in the cache dir
			checkoutPath, found := a.ResponseStringAnnotation(ins.ID(), "git-checkout-path")
			c.Assert(found, Equals, true)
			prefix := a.SessionCacheDir + "/git-"
			c.Assert(strings.HasPrefix(checkoutPath, prefix), Equals, true)
			stat, err := os.Stat(checkoutPath)
			c.Assert(err, IsNil)
			c.Assert(stat.IsDir(), Equals, true)
		} else {
			c.Assert(a.Rejected(), Equals, true)
			c.Assert(a.ResponseInspection[ins.ID()].Annotations["error-msg"], Equals, tc.errmsg, Commentf("test case: +%v", tc))
		}

		c.Check(a.Metadata.Type, Equals, "application/x.git.upload-pack-result.fetch")
	}
}
