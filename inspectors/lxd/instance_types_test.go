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
 *
 */

package lxd_test

import (
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/lxd"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type lxdInstanceTypesSuite struct {
}

func (t *lxdInstanceTypesSuite) SetUpTest(c *C) {}

var _ = Suite(&lxdInstanceTypesSuite{})

func (s *lxdInstanceTypesSuite) TestInstanceTypesInspectorID(c *C) {
	ins := lxd.NewInstanceTypesInspector()
	c.Assert(ins.ID(), Equals, "lxd.instance-types")
}

type lxdInstanceTypesInspectRequestTest struct {
	url     string // The info request URL
	pending bool   // Whether the inspection result should be pending
}

var lxdInstanceTypesInspectRequestTests = []lxdInstanceTypesInspectRequestTest{{
	url:     "https://images.lxd.canonical.com:443/meta/instance-types/name.yaml",
	pending: true,
}, {
	url:     "https://images.lxd.canonical.com:443/meta/instance-types/.yaml",
	pending: true,
}, {
	url:     "http://images.lxd.canonical.com/meta/instance-types/all.yaml",
	pending: false,
}, {
	url:     "https://not-images.lxd.canonical.com:443/meta/instance-types/all.yaml",
	pending: false,
}}

func (s *lxdInstanceTypesSuite) TestInstanceTypesInspectRequest(c *C) {
	for _, tc := range lxdInstanceTypesInspectRequestTests {
		ins := lxd.NewInstanceTypesInspector()
		a := metadata.NewArtifact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, tc.pending, Commentf("test case: %+v", tc))
		if tc.pending {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		}
	}
}

type lxdInstanceTypesArtifactInspectorTest struct {
	filename string // The test file name
	pending  bool   // Whether the request was set to pending
	approved bool   // Whether the artifact should be approved
	reason   string // The reason for approval or rejection
}

var lxdInstanceTypesArtifactInspectorTests = []lxdInstanceTypesArtifactInspectorTest{{
	filename: "testdata/instance-types-all.yaml",
	pending:  true,
	approved: true,
	reason:   "valid LXD instance types metadata",
}, {
	filename: "testdata/instance-types-aws.yaml",
	pending:  true,
	approved: true,
	reason:   "valid LXD instance types metadata",
}, {
	filename: "testdata/instance-types-index.yaml",
	pending:  true,
	approved: true,
	reason:   "valid LXD instance types metadata",
}, {
	filename: "testdata/instance-types-aws-bad.yaml",
	pending:  true,
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/instance-types-index-bad.yaml",
	pending:  true,
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/empty.txt",
	pending:  true,
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/index.json",
	pending:  false,
	approved: false,
	reason:   "", // unrecognized artifact
}, {
	filename: "testdata/instance-types-all.yaml",
	pending:  false,
	approved: false,
	reason:   "", // unrecognized artifact
}}

func (s *lxdInstanceTypesSuite) TestInstanceTypesArtifactInspector(c *C) {
	for _, tc := range lxdInstanceTypesArtifactInspectorTests {
		a := metadata.NewArtifact()
		a.Metadata.Type = "text/plain"
		a.Metadata.Size = 1234

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)
		defer f.Close()

		ins := lxd.NewInstanceTypesInspector()
		if tc.pending {
			a.SetRequestPending(ins, "test")
		}
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Check(a.Approved(), Equals, tc.approved, Commentf("%+v", tc))

		if tc.approved {
			insp := a.ResponseInspection["lxd.instance-types"]
			c.Assert(insp, Not(IsNil))
			c.Check(insp.Opinion, Equals, opinions.Approved)

			c.Check(insp.Reason, Equals, tc.reason)
			c.Check(a.Metadata.Type, Equals, "application/x.canonical.lxd-instance-types")
			c.Check(a.Metadata.Name, Equals, "Instance types")
			c.Check(a.Metadata.Description, Equals, "LXD instance types metadata")
		}
	}
}

func (s *lxdInstanceTypesSuite) TestInstanceTypesArtifactBadType(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/json"
	a.Metadata.Size = 1743

	f, err := files.OpenArtifactFile("testdata/instance-types-all.yaml")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := lxd.NewInstanceTypesInspector()
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Check(a.Approved(), Equals, false)
}
