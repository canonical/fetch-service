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

package lxd_test

import (
	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/lxd"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type rootfsSuite struct{}

func (t *rootfsSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&rootfsSuite{})

func (s *rootfsSuite) TestRootfsInspectorInterface(c *C) {
	var iface Inspector
	ins := lxd.NewRootfsInspector()
	c.Assert(ins, Implements, &iface)

}

func (s *rootfsSuite) TestRootfsInspectorID(c *C) {
	ins := lxd.NewRootfsInspector()
	c.Assert(ins.ID(), Equals, "lxd.rootfs")

}

type inspectRequestTest struct {
	url     string // The request URL
	pending bool   // The expected result of the request inspection
}

var inspectRequestTests = []inspectRequestTest{{
	url:     "http://cloud-images.ubuntu.com/buildd/daily/noble/20250629/noble-server-cloudimg-amd64-lxd_combined.tar.gz",
	pending: true,
}, {
	url:     "https://cloud-images.ubuntu.com:443/buildd/daily/noble/20250629/noble-server-cloudimg-amd64-lxd_combined.tar.gz",
	pending: true,
}, {
	// Not a daily image
	url:     "http://cloud-images.ubuntu.com/buildd/not-daily/noble/20250629/noble-server-cloudimg-amd64-lxd_combined.tar.gz",
	pending: false,
}, {
	// Not a tar.gz file
	url:     "http://cloud-images.ubuntu.com/buildd/daily/noble/20250629/noble-server-cloudimg-amd64-lxd_combined.tar.xz",
	pending: false,
}, {
	// Invalid origin
	url:     "http://not-cloud-images.ubuntu.com/buildd/daily/noble/20250629/noble-server-cloudimg-amd64-lxd_combined.tar.gz",
	pending: false,
}, {
	// Missing file name
	url:     "http://cloud-images.ubuntu.com/buildd/daily/noble/20250629",
	pending: false,
}}

func (s *rootfsSuite) TestInspectRequest(c *C) {
	for _, tc := range inspectRequestTests {
		ins := lxd.NewRootfsInspector()
		a := metadata.NewArtifact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, tc.pending)
		if tc.pending {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
			c.Assert(insp.Reason, Equals, "valid URL for lxd rootfs")
			c.Assert(insp.Annotations, DeepEquals, Annotation{
				"image-series": "noble",
				"image-date":   "20250629",
				"image-name":   "noble-server-cloudimg-amd64-lxd_combined.tar.gz",
			})
		}
	}
}

func (s *rootfsSuite) TestInspectArtifact(c *C) {
	ins := lxd.NewRootfsInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/gzip")
	a.SetRequestPending(ins, "test")

	f, err := files.OpenArtifactFile("testdata/base.tar.gz")
	c.Assert(err, IsNil)

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, true)
	c.Check(a.Metadata.Type, Equals, "application/x.canonical.lxd.rootfs")
	c.Check(a.Metadata.Name, Equals, "LXD rootfs image")
	c.Check(a.Metadata.Version, Equals, "1751220536")
	c.Check(a.Metadata.Description, Equals, "Ubuntu buildd noble amd64")
	c.Check(a.Metadata.Architecture, Equals, "amd64")
	c.Check(a.ResponseInspection["lxd.rootfs"], DeepEquals, &Inspection{
		Opinion: opinions.Approved,
		Reason:  "valid LXD rootfs tarball",
		Annotations: Annotation{
			"architecture":  "x86_64",
			"os":            "Ubuntu",
			"series":        "noble",
			"creation-date": int64(1751220536),
		},
	})
}

func (s *rootfsSuite) TestInspectArtifactBadType(c *C) {
	ins := lxd.NewRootfsInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/zip")

	err := ins.InspectArtifact(nil, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
	c.Assert(a.Rejected(), Equals, true)
}

type inspectArtifactBadContentTest struct {
	filename string // Name of the test file
}

var inspectArtifactBadContentTests = []inspectArtifactBadContentTest{{
	filename: "testdata/base-no-metadata.tar.gz",
}, {
	filename: "testdata/base-no-rootfs.tar.gz",
}, {
	filename: "testdata/index.json",
}}

func (s *rootfsSuite) TestInspectArtifactSkip(c *C) {
	for _, tc := range inspectArtifactBadContentTests {
		ins := lxd.NewRootfsInspector()
		a := metadata.NewArtifact()
		a.MimeType = mimetype.Lookup("application/gzip")
		a.SetRequestPending(ins, "test")

		f, err := files.OpenArtifactFile(tc.filename)
		c.Assert(err, IsNil)

		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Assert(a.Approved(), Equals, false)

		// No opinions recorded
		_, ok := a.ResponseInspection[ins.ID()]
		c.Assert(ok, Equals, false)
	}
}
