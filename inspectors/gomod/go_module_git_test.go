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

package gomod_test

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/gomod"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type goModuleGitSuite struct{}

var _ = Suite(&goModuleGitSuite{})

func Test(t *testing.T) { TestingT(t) }

func (t *goModuleGitSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

func (s *goModuleGitSuite) TestGoModuleGitInspectorInterface(c *C) {
	var iface Inspector
	ins := gomod.NewGoModuleGitInspector()
	c.Assert(ins, Implements, &iface)

}

func (s *goModuleGitSuite) TestGoModuleGitInspectorID(c *C) {
	ins := gomod.NewGoModuleGitInspector()
	c.Assert(ins.ID(), Equals, "go.module.git")

}

func (s *goModuleGitSuite) TestInspectGoModuleGitRequest(c *C) {
	for _, tc := range []struct {
		url      string
		approved bool
	}{
		// FIXME: using github as placeholder, final URLs will change
		{"https://github.com:443/user/project.git/git-upload-pack", true},
		{"https://github.com:443/user/project/git-upload-pack", true},
		{"https://git.launchpad.net:443/project/git-upload-pack", true},
		{"https://git.launchpad.net:443/~user/project/+git/project/git-upload-pack", true},
		{"https://gopkg.in:443/project.v2/git-upload-pack", true},
		{"https://invalid.com:443/user/project.git/git-upload-pack", false},
		{"http://github.com/user/project.git/git-upload-pack", false},
		{"https://gothub.com:443/user/project.git/git-upload-pack", false},
		{"ahttps://github.com:443/user/project.git/git-upload-pack", false},
		{"https://github.com:443/user/project.git/git-upload-packs", false},
		{"https://github.com:443/user/project.git/something-else", false},
		{"https://git.launchpad.com:443/project/git-upload-pack", false},
		{"https://git.lpad.net:443/~user/project/+git/project/git-upload-pack", false},
	} {
		ins := gomod.NewGoModuleGitInspector()
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
		c.Assert(ok, Equals, tc.approved)
		if tc.approved {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		}
	}
}

/*
00000000  30 30 30 64 70 61 63 6b  66 69 6c 65 0a 30 31 32  |000dpackfile.012|
00000010  34 01 50 41 43 4b 00 00  00 02 00 00 00 03 96 0f  |4.PACK..........|
00000020  78 9c 9d cc 3b 0e 02 21  14 40 d1 9e 55 bc 0d 60  |x...;..!.@..U..`|
00000030  f8 0c 08 c6 18 13 6b 2b  57 f0 e4 33 be 08 43 32  |......k+W..3..C2|
00000040  42 e1 ee 35 d1 05 18 bb  9b 5b 9c be a6 04 56 09  |B..5.....[....V.|
00000050  a1 b2 0d 68 d5 94 73 4c  4e 3b e3 8d 0f ca 4d e8  |...h..sLN;....M.|
00000060  ac 34 da 07 0c 09 33 c3  d1 6f 6d 85 53 c1 11 a9  |.4....3..om.S...|
00000070  c1 19 fb 63 b4 3b c2 3e  d4 6f 1e e7 8a 54 36 a1  |...c.;.>.o...T6.|
00000080  d5 03 c8 ad 54 46 0b 25  35 70 a1 85 60 ef 5b a9  |....TF.%5p..`.[.|
00000090  f7 f4 bf c0 68 a1 4e 58  e0 43 31 76 a1 79 49 91  |....h.NX.C1v.yI.|
000000a0  b7 9c f9 f5 b9 fb d1 65  2f 7b 9c 4f 83 a2 02 78  |.......e/{.O...x|
000000b0  9c 33 34 30 30 33 31 51  48 cf d7 cb cd 4f 61 e8  |.340031QH....Oa.|
000000c0  0a 5e 2e b0 c1 aa db ed  d4 9d fd be 8b 4e 9d be  |.^...........N..|
000000d0  b6 fb ea 81 ed 00 cf d0  0f a9 be 03 78 9c cb cd  |............x...|
000000e0  4f 29 cd 49 55 48 ad 48  cc 2d c8 49 d5 4b ce cf  |O).IUH.H.-.I.K..|
000000f0  d5 2f 49 2d 2e 01 12 b9  05 f9 45 89 45 95 ba 20  |./I-......E.E.. |
00000100  2e 17 57 7a be 82 91 9e  b1 09 17 57 51 6a 61 69  |..Wz.......WQjai|
00000110  66 51 6a b1 82 06 97 26  17 00 ce 20 14 c8 11 35  |fQj....&... ...5|
00000120  d4 cc 9e 5a 6f a6 0b 5c  c9 91 ea 5d c4 5e 99 bf  |...Zo..\...].^..|
00000130  84 30 30 30 36 01 3f 30  30 30 30                 |.0006.?0000|
*/

var goModuleGitFetch = []byte{
	0x30, 0x30, 0x30, 0x64, 0x70, 0x61, 0x63, 0x6b, 0x66, 0x69, 0x6c, 0x65, 0x0a, 0x30, 0x31, 0x32,
	0x34, 0x01, 0x50, 0x41, 0x43, 0x4b, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x03, 0x96, 0x0f,
	0x78, 0x9c, 0x9d, 0xcc, 0x3b, 0x0e, 0x02, 0x21, 0x14, 0x40, 0xd1, 0x9e, 0x55, 0xbc, 0x0d, 0x60,
	0xf8, 0x0c, 0x08, 0xc6, 0x18, 0x13, 0x6b, 0x2b, 0x57, 0xf0, 0xe4, 0x33, 0xbe, 0x08, 0x43, 0x32,
	0x42, 0xe1, 0xee, 0x35, 0xd1, 0x05, 0x18, 0xbb, 0x9b, 0x5b, 0x9c, 0xbe, 0xa6, 0x04, 0x56, 0x09,
	0xa1, 0xb2, 0x0d, 0x68, 0xd5, 0x94, 0x73, 0x4c, 0x4e, 0x3b, 0xe3, 0x8d, 0x0f, 0xca, 0x4d, 0xe8,
	0xac, 0x34, 0xda, 0x07, 0x0c, 0x09, 0x33, 0xc3, 0xd1, 0x6f, 0x6d, 0x85, 0x53, 0xc1, 0x11, 0xa9,
	0xc1, 0x19, 0xfb, 0x63, 0xb4, 0x3b, 0xc2, 0x3e, 0xd4, 0x6f, 0x1e, 0xe7, 0x8a, 0x54, 0x36, 0xa1,
	0xd5, 0x03, 0xc8, 0xad, 0x54, 0x46, 0x0b, 0x25, 0x35, 0x70, 0xa1, 0x85, 0x60, 0xef, 0x5b, 0xa9,
	0xf7, 0xf4, 0xbf, 0xc0, 0x68, 0xa1, 0x4e, 0x58, 0xe0, 0x43, 0x31, 0x76, 0xa1, 0x79, 0x49, 0x91,
	0xb7, 0x9c, 0xf9, 0xf5, 0xb9, 0xfb, 0xd1, 0x65, 0x2f, 0x7b, 0x9c, 0x4f, 0x83, 0xa2, 0x02, 0x78,
	0x9c, 0x33, 0x34, 0x30, 0x30, 0x33, 0x31, 0x51, 0x48, 0xcf, 0xd7, 0xcb, 0xcd, 0x4f, 0x61, 0xe8,
	0x0a, 0x5e, 0x2e, 0xb0, 0xc1, 0xaa, 0xdb, 0xed, 0xd4, 0x9d, 0xfd, 0xbe, 0x8b, 0x4e, 0x9d, 0xbe,
	0xb6, 0xfb, 0xea, 0x81, 0xed, 0x00, 0xcf, 0xd0, 0x0f, 0xa9, 0xbe, 0x03, 0x78, 0x9c, 0xcb, 0xcd,
	0x4f, 0x29, 0xcd, 0x49, 0x55, 0x48, 0xad, 0x48, 0xcc, 0x2d, 0xc8, 0x49, 0xd5, 0x4b, 0xce, 0xcf,
	0xd5, 0x2f, 0x49, 0x2d, 0x2e, 0x01, 0x12, 0xb9, 0x05, 0xf9, 0x45, 0x89, 0x45, 0x95, 0xba, 0x20,
	0x2e, 0x17, 0x57, 0x7a, 0xbe, 0x82, 0x91, 0x9e, 0xb1, 0x09, 0x17, 0x57, 0x51, 0x6a, 0x61, 0x69,
	0x66, 0x51, 0x6a, 0xb1, 0x82, 0x06, 0x97, 0x26, 0x17, 0x00, 0xce, 0x20, 0x14, 0xc8, 0x11, 0x35,
	0xd4, 0xcc, 0x9e, 0x5a, 0x6f, 0xa6, 0x0b, 0x5c, 0xc9, 0x91, 0xea, 0x5d, 0xc4, 0x5e, 0x99, 0xbf,
	0x84, 0x30, 0x30, 0x30, 0x36, 0x01, 0x3f, 0x30, 0x30, 0x30, 0x30,
}

func (s *goModuleGitSuite) TestGoModuleGitInspectArtefact(c *C) {
	for _, tc := range []struct {
		data        []byte
		is_shallow  bool
		has_version bool
		opinion     opinions.OpinionKind
		reason      string
	}{
		{goModuleGitFetch, true, true, opinions.Approved, "go module found"},
		{goModuleGitFetch, false, true, opinions.Rejected, "go module found but repository is not shallow"},
		{goModuleGitFetch, true, false, opinions.Rejected, "cannot find go module version tag"},
	} {
		a := metadata.NewArtefact()
		a.Request, _ = http.NewRequest("GET", "https://example.com:443/test/git-upload-pack", nil)
		a.CurrentDownload.ContentType = "application/x-git-upload-pack-result"
		a.Request.Body = io.NopCloser(strings.NewReader("0014command=fetch\n0000"))
		a.MimeType = mimetype.Lookup("application/octet-stream")
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
						"want 467ef24fabbcce4a3bda7af3918fb970ee970c8b",
						"done",
					},
					"repository": "https://my.repo/foo",
					"command":    "fetch",
					"project":    "bump2version",
					"protocol":   "version=2",
					"wants": []string{
						"467ef24fabbcce4a3bda7af3918fb970ee970c8b",
					},
					"is-shallow": tc.is_shallow,
				},
			},
		}
		if tc.has_version {
			a.ResponseInspection = metadata.InspectionMap{
				"git.upload-pack": &Inspection{
					Opinion: opinions.Unknown,
					Reason:  "",
					Annotations: Annotation{
						"tags": map[string]string{
							"v1.0": "467ef24fabbcce4a3bda7af3918fb970ee970c8b",
						},
					},
				},
			}
		}

		f := bytes.NewReader(tc.data)

		ins := gomod.NewGoModuleGitInspector()
		err := ins.InspectArtefact(f, a)
		c.Assert(err, IsNil)

		inspection := a.ResponseInspection["go.module.git"]
		c.Assert(inspection.Opinion, Equals, tc.opinion)
		c.Assert(inspection.Reason, Equals, tc.reason)
	}
}
