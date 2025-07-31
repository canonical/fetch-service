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
	"strings"
	"testing"

	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/lxd"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/gabriel-vasile/mimetype"
)

type simpleStreamIndexSuite struct {
	slog logger.Logger
}

func (t *simpleStreamIndexSuite) SetUpTest(c *C) {}

var _ = Suite(&simpleStreamIndexSuite{logger.NewSessionLogger("test")})

func Test(t *testing.T) { TestingT(t) }

func (s *simpleStreamIndexSuite) TestSimpleStreamsIndexInspectorInterface(c *C) {
	var iface Inspector
	ins := lxd.NewSimpleStreamsIndexInspector()
	c.Assert(ins, Implements, &iface)

}

func (s *simpleStreamIndexSuite) TestSimpleStreamsIndexInspectorID(c *C) {
	ins := lxd.NewSimpleStreamsIndexInspector()
	c.Assert(ins.ID(), Equals, "lxd.simple-streams.index")
}

type simpleStreamIndexInspectRequestTest struct {
	url     string
	pending bool
	stream  string
}

var simpleStreamIndexInspectRequestTests = []simpleStreamIndexInspectRequestTest{
	{
		url:     "https://cloud-images.ubuntu.com:443/daily/streams/v1/index.json",
		pending: true,
		stream:  "daily",
	},
	{
		url:     "https://cloud-images.ubuntu.com:443/releases/streams/v1/index.json",
		pending: true,
		stream:  "releases",
	},
	{
		url:     "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/index.json",
		pending: true,
		stream:  "buildd/daily",
	},
	{
		url:     "https://example.com/streams/v1/index.json",
		pending: false,
		stream:  "",
	},
	{
		url:     "https://cloud-images.ubuntu.com:443/daily/streams/v1/other.json",
		pending: false,
		stream:  "",
	},
	{
		url:     "https://cloud-images.ubuntu.com:443/daily/index.json",
		pending: false,
		stream:  "",
	},
}

func (s *simpleStreamIndexSuite) TestSimpleStreamsIndexInspectorInspectRequest(c *C) {
	ins := lxd.NewSimpleStreamsIndexInspector()

	for _, test := range simpleStreamIndexInspectRequestTests {
		a := metadata.NewArtifact()
		a.CurrentDownload = metadata.Download{URL: test.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp := a.RequestInspection[ins.ID()]
		c.Assert(a.RequestPending(), Equals, test.pending)

		if test.pending {
			c.Assert(insp.Reason, Equals, "valid Simple Streams index URL")
			c.Assert(insp.Opinion, Equals, opinions.Pending)
			stream, ok := insp.Annotations["stream"]
			c.Assert(ok, Equals, true, Commentf("Stream annotation should be present for %s", test.url))
			c.Assert(stream, Equals, test.stream, Commentf("Stream should match for %s", test.url))
		}
	}
}

func (s *simpleStreamIndexSuite) TestSimpleStreamsIndexInspectorInspectArtifact(c *C) {
	ins := lxd.NewSimpleStreamsIndexInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/json")
	a.CurrentDownload = metadata.Download{URL: "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/index.json"}

	// Set up the request inspection first (required for InspectArtifact)
	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	c.Assert(a.RequestPending(), Equals, true)

	f, err := files.OpenArtifactFile("testdata/index.json")
	c.Assert(err, IsNil)
	defer f.Close()

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, true)
	c.Check(a.Metadata.Type, Equals, mimetypes.SimpleStreams)
	c.Check(a.Metadata.Name, Equals, "Simple Streams Index")
	c.Check(a.Metadata.Description, Equals, "Simple Streams Index for buildd/daily")

	insp, ok := a.ResponseInspection[ins.ID()]
	c.Assert(ok, Equals, true)
	c.Assert(insp.Opinion, Equals, opinions.Approved)

	downloadPaths, ok := insp.Annotations["download-paths"]
	c.Assert(ok, Equals, true)
	expectedPaths := []string{"streams/v1/com.ubuntu.cloud:daily:download.json"}
	c.Assert(downloadPaths, DeepEquals, expectedPaths)
}

func (s *simpleStreamIndexSuite) TestSimpleStreamsIndexInspectorInspectArtifactWrongMimetype(c *C) {
	ins := lxd.NewSimpleStreamsIndexInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/plain")
	a.CurrentDownload = metadata.Download{URL: "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/index.json"}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	c.Assert(a.RequestPending(), Equals, true)

	f := strings.NewReader(`{"format": "index:1.0"}`)
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
}

func (s *simpleStreamIndexSuite) TestSimpleStreamsIndexInspectorInspectArtifactNoRequestAnnotation(c *C) {
	ins := lxd.NewSimpleStreamsIndexInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/json")

	f, err := files.OpenArtifactFile("testdata/index.json")
	c.Assert(err, IsNil)
	defer f.Close()
	c.Assert(a.RequestPending(), Equals, false)

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
}

func (s *simpleStreamIndexSuite) TestInvalidIndexFormat(c *C) {
	ins := lxd.NewSimpleStreamsIndexInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/json")
	a.CurrentDownload = metadata.Download{URL: "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/index.json"}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	c.Assert(a.RequestPending(), Equals, true)

	f := strings.NewReader(`{"format": "index:2.0"}`)
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
	_, ok := a.ResponseInspection[ins.ID()]
	c.Assert(ok, Equals, false)
}

type simpleStreamIndexInvalidArtifactTest struct {
	json        string
	annotations map[string]any
}

var simpleStreamIndexInvalidArtifactTests = []simpleStreamIndexInvalidArtifactTest{
	{
		json: `{
		"format": "index:1.0",
		"index": {
			"test": {
				"datatype": "invalid-type",
				"format": "products:1.0",
				"path": "test.json"
			}
		}
	}`,
		annotations: map[string]any{"index.datatype": "invalid-type"},
	},
	{
		json: `{
		"format": "index:1.0",
		"index": {
			"test": {
				"datatype": "image-downloads",
				"format": "products:2.0",
				"path": "test.json"
			}
		}
	}`,
		annotations: map[string]any{"index.format": "products:2.0"},
	},
}

func (s *simpleStreamIndexSuite) TestSimpleStreamsIndexInspectorInspectArtifactInvalidFormat(c *C) {
	ins := lxd.NewSimpleStreamsIndexInspector()

	for _, test := range simpleStreamIndexInvalidArtifactTests {
		a := metadata.NewArtifact()
		a.MimeType = mimetype.Lookup("application/json")
		a.CurrentDownload = metadata.Download{URL: "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/index.json"}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)
		c.Assert(a.RequestPending(), Equals, true)

		f := strings.NewReader(test.json)
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)
		c.Assert(a.Rejected(), Equals, true)

		c.Check(a.ResponseInspection, DeepEquals, metadata.InspectionMap{
			"lxd.simple-streams.index": &Inspection{
				Opinion:     opinions.Rejected,
				Reason:      "invalid index file",
				Annotations: test.annotations,
			},
		})
	}
}

func (s *simpleStreamIndexSuite) TestSimpleStreamsIndexInspectorInspectArtifactNotJSON(c *C) {
	ins := lxd.NewSimpleStreamsIndexInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/json")
	a.CurrentDownload = metadata.Download{URL: "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/index.json"}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	c.Assert(a.RequestPending(), Equals, true)

	f := strings.NewReader(`bad json`)
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	_, ok := a.ResponseInspection[ins.ID()]
	c.Assert(ok, Equals, false)
}
