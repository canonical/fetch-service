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
	"strings"
	"testing"

	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/git"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type smartQuerySuite struct{}

var _ = Suite(&smartQuerySuite{})

func (t *smartQuerySuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

func Test(t *testing.T) { TestingT(t) }

func (s *smartQuerySuite) TestSmartQueryInspectorInterface(c *C) {
	var iface Inspector
	ins := git.NewSmartQueryInspector()
	c.Assert(ins, Implements, &iface)

}

func (s *smartQuerySuite) TestSmartQueryInspectorID(c *C) {
	ins := git.NewSmartQueryInspector()
	c.Assert(ins.ID(), Equals, "git.smart-query")

}

func (s *smartQuerySuite) TestInspectRequest(c *C) {
	for _, tc := range []struct {
		url      string
		approved bool
	}{
		// FIXME: using github as placeholder, final URLs will change
		{"https://github.com:443/user/project.git/info/refs?service=git-upload-pack", true},
		{"https://git.launchpad.net:443/project/info/refs?service=git-upload-pack", true},
		{"https://git.launchpad.net:443/~user/project/+git/project/info/refs?service=git-upload-pack", true},
		{"http://github.com/user/project.git/info/refs?service=git-upload-pack", false},
		{"ahttps://github.com:443/user/project.git/info/refs?service=git-upload-pack", false},
		{"https://github.com:443/user/project.git/info/refs?service=git-upload-packs", false},
		{"https://github.com:443/user/project.git/info/refs?service=something-else", false},
		{"https://github.com:443/user/project.git/info/refs", false},
		{"https://git.launchpad.com:443/project/info/refs?service=git-upload-pack", false},
		{"https://git.lpad.net:443/~user/project/+git/project/info/refs", false},
	} {
		ins := git.NewSmartQueryInspector()
		a := fakeGitArtefact()
		a.CurrentDownload.URL = tc.url

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, tc.approved)
		if tc.approved {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		}
	}
}

var smartQueryArtefactData = `001e# service=git-upload-pack
0000000eversion 2
0022agent=git/github-de75ed0166ec
0013ls-refs=unborn
0027fetch=shallow wait-for-done filter
0012server-option
0017object-format=sha1
0000`

func (s *smartQuerySuite) TestSmartQueryInspectArtefact(c *C) {
	for _, tc := range []struct {
		data   string
		result bool
	}{
		{smartQueryArtefactData, true},
		{smartQueryArtefactData[1:], false},
		{smartQueryArtefactData[:len(smartQueryArtefactData)-1], false},
	} {
		a := fakeGitArtefact()
		a.CurrentDownload.ContentType = "application/x-git-upload-pack-advertisement"

		f := strings.NewReader(tc.data)

		ins := git.NewSmartQueryInspector()
		a.SetRequestPending(ins, "test")
		err := ins.InspectArtefact(f, a)
		c.Assert(err, IsNil)

		c.Check(a.Metadata.Type, Equals, "application/x.git.upload-pack-advertisement")
		c.Check(a.Metadata.Name, Equals, "git upload-pack advertisement")
		c.Assert(a.Approved(), Equals, tc.result)
	}
}
