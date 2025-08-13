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

	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/lxd"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/gabriel-vasile/mimetype"
)

type simpleStreamDownloadSuite struct {
	slog logger.Logger
}

func (t *simpleStreamDownloadSuite) SetUpTest(c *C) {}

var _ = Suite(&simpleStreamDownloadSuite{logger.NewSessionLogger("test")})

func (s *simpleStreamDownloadSuite) TestSimpleDownloadInspectorInterface(c *C) {
	var iface Inspector
	ins := lxd.NewSimpleStreamsDownloadInspector()
	c.Assert(ins, Implements, &iface)

}

func (s *simpleStreamDownloadSuite) TestSimpleDownloadInspectorID(c *C) {
	ins := lxd.NewSimpleStreamsDownloadInspector()
	c.Assert(ins.ID(), Equals, "lxd.simple-streams.download")
}

type simpleStreamDownloadInspectRequestTest struct {
	url     string
	pending bool
	stream  string
}

var simpleStreamDownloadInspectRequestTests = []simpleStreamDownloadInspectRequestTest{
	{
		url:     "https://cloud-images.ubuntu.com:443/daily/streams/v1/com.ubuntu.cloud:daily:download.json",
		pending: true,
		stream:  "daily",
	},
	{
		url:     "https://cloud-images.ubuntu.com:443/releases/streams/v1/com.ubuntu.cloud:releases:download.json",
		pending: true,
		stream:  "releases",
	},
	{
		url:     "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/com.ubuntu.cloud:daily:download.json",
		pending: true,
		stream:  "buildd/daily",
	},
	{
		url:     "https://example.com:443/streams/v1/download.json",
		pending: false,
		stream:  "",
	},
	{
		url:     "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/com.ubuntu.cloud:daily:other.json",
		pending: false,
		stream:  "",
	},
}

func (s *simpleStreamDownloadSuite) TestSimpleDownloadInspectorInspectRequest(c *C) {
	ins := lxd.NewSimpleStreamsDownloadInspector()

	for _, test := range simpleStreamDownloadInspectRequestTests {
		a := metadata.NewArtifact()
		a.CurrentDownload = metadata.Download{URL: test.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp := a.RequestInspection[ins.ID()]
		c.Assert(a.RequestPending(), Equals, test.pending, Commentf("%s", test.url))

		if test.pending {
			c.Assert(insp.Reason, Equals, "valid Simple Streams download request URL")
			c.Assert(insp.Opinion, Equals, opinions.Pending)
			stream, ok := insp.Annotations["stream"]
			c.Assert(ok, Equals, true, Commentf("Stream annotation should be present for %s", test.url))
			c.Assert(stream, Equals, test.stream, Commentf("Stream should match for %s", test.url))
		}
	}
}

func (s *simpleStreamDownloadSuite) TestSimpleDownloadInspectorInspectArtifact(c *C) {
	ins := lxd.NewSimpleStreamsDownloadInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/json")
	a.CurrentDownload = metadata.Download{URL: "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/com.ubuntu.cloud:daily:download.json"}

	// Set up the request inspection first (required for InspectArtifact)
	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	c.Assert(a.RequestPending(), Equals, true)

	f, err := files.OpenArtifactFile("testdata/download.json")
	c.Assert(err, IsNil)
	defer f.Close()

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, true)
	c.Check(a.Metadata.Type, Equals, "application/x.canonical.simplestreams-products")
	c.Check(a.Metadata.Name, Equals, "Simple Streams Download")
	c.Check(a.Metadata.Description, Equals, "Simple Streams Download for com.ubuntu.cloud:daily:download")

	insp, ok := a.ResponseInspection[ins.ID()]
	c.Assert(ok, Equals, true)
	c.Assert(insp.Opinion, Equals, opinions.Approved)
	images := insp.Annotations["product-items"].(map[string]string)
	// Check for 1 value in images
	sha256, ok := images["jammy/20250621/jammy-server-cloudimg-amd64-lxd_combined.tar.gz"]
	c.Assert(ok, Equals, true)
	c.Assert(sha256, Equals, "b30a183187a391e87c7752a4e52724f6cc66ddcc875f8e842d1937680f243d8c")

	// The image
	ad := metadata.NewArtifact()
	ad.CurrentDownload = metadata.Download{
		URL: "https://cloud-images.ubuntu.com:443/buildd/daily/jammy/20250621/jammy-server-cloudimg-amd64-lxd_combined.tar.gz",
	}
	expectedSha256, _ := digests.NewSha256Digest("b30a183187a391e87c7752a4e52724f6cc66ddcc875f8e842d1937680f243d8c")
	ad.Metadata.Sha256 = expectedSha256
	ad.MimeType = mimetype.Lookup("application/gzip")

	// Set up the request inspection first (required for InspectArtifact)
	err = ins.InspectRequest(ad)
	c.Assert(err, IsNil)
	c.Assert(ad.RequestPending(), Equals, true)

	err = ins.InspectArtifact(strings.NewReader("not json"), ad)
	c.Assert(err, IsNil)

	// The transactional inspection does not approve, but can reject
	adInsp, ok := ad.ResponseInspection[ins.ID()]
	c.Check(ok, Equals, true)
	c.Check(adInsp.Opinion, Equals, opinions.Unknown)
	c.Check(adInsp.Reason, Equals, "simple streams product item matches digest")
	c.Check(adInsp.Annotations, DeepEquals, Annotation{
		"product-item-path": "jammy/20250621/jammy-server-cloudimg-amd64-lxd_combined.tar.gz",
		"sha256":            "b30a183187a391e87c7752a4e52724f6cc66ddcc875f8e842d1937680f243d8c",
	})
}

func (s *simpleStreamDownloadSuite) TestSimpleDownloadInspectorInspectArtifactNonJSON(c *C) {
	ins := lxd.NewSimpleStreamsDownloadInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("text/plain")
	a.CurrentDownload = metadata.Download{URL: "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/com.ubuntu.cloud:daily:download.json"}

	// Set up the request inspection first
	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	c.Assert(a.RequestPending(), Equals, true)

	f, err := files.OpenArtifactFile("testdata/download.json")
	c.Assert(err, IsNil)
	defer f.Close()

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
	c.Check(a.Metadata.Type, Equals, "")
}

func (s *simpleStreamDownloadSuite) TestSimpleDownloadInspectorInspectArtifactMissingStreamAnnotation(c *C) {
	ins := lxd.NewSimpleStreamsDownloadInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/json")
	a.CurrentDownload = metadata.Download{URL: "https://example.com/other.json"}

	// Don't set up request inspection, so no stream annotation exists
	f, err := files.OpenArtifactFile("testdata/download.json")
	c.Assert(err, IsNil)
	defer f.Close()

	err = ins.InspectArtifact(f, a)
	c.Assert(err, ErrorMatches, "missing stream in request annotations")
	c.Assert(a.Approved(), Equals, false)
	c.Check(a.Metadata.Type, Equals, "")
}

func (s *simpleStreamDownloadSuite) TestSimpleDownloadInspectorInspectArtifactInvalidJSON(c *C) {
	ins := lxd.NewSimpleStreamsDownloadInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/json")
	a.CurrentDownload = metadata.Download{URL: "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/com.ubuntu.cloud:daily:download.json"}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	c.Assert(a.RequestPending(), Equals, true)

	f := strings.NewReader(`{"updated": "2023-01-01T00:00:00Z", "format": "products:1.0", "invalid_json": "missing closing brace"`)

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
	c.Check(a.Metadata.Type, Equals, "")
}

func (s *simpleStreamDownloadSuite) TestSimpleDownloadInspectorInspectArtifactUnsupportedFormat(c *C) {
	ins := lxd.NewSimpleStreamsDownloadInspector()
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/json")
	a.CurrentDownload = metadata.Download{URL: "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/com.ubuntu.cloud:daily:download.json"}

	// Set up the request inspection first
	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	c.Assert(a.RequestPending(), Equals, true)

	f := strings.NewReader(`{"updated": "2023-01-01T00:00:00Z", "format": "unsupported:1.0", "datatype": "image-downloads", "content_id": "com.ubuntu.cloud:daily:download"}`)

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)
	c.Assert(a.Approved(), Equals, false)
	c.Check(a.Metadata.Type, Equals, "")
}

func (s *simpleStreamDownloadSuite) TestSimpleDownloadInspectorInspectImageMissingSha256(c *C) {
	ins := lxd.NewSimpleStreamsDownloadInspector()

	// First process a download JSON to populate the images map
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/json")
	a.CurrentDownload = metadata.Download{URL: "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/com.ubuntu.cloud:daily:download.json"}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)

	f, err := files.OpenArtifactFile("testdata/download.json")
	c.Assert(err, IsNil)
	defer f.Close()

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	// Now test non existent image
	ad := metadata.NewArtifact()
	ad.CurrentDownload = metadata.Download{URL: "https://cloud-images.ubuntu.com:443/buildd/daily/jammy/20250621/no-image-found.img"}
	ad.MimeType = mimetype.Lookup("application/gzip")
	expectedSha256, _ := digests.NewSha256Digest("2d06e9092ec19fbe3c04402a0c1a53ae7c0b1079b041e5b5988c8febaabe25d3")
	ad.Metadata.Sha256 = expectedSha256

	err = ins.InspectRequest(ad)
	c.Assert(err, IsNil)
	// this will be false as we have no record of the image
	c.Assert(ad.RequestPending(), Equals, false)

	err = ins.InspectArtifact(strings.NewReader("some unknown image"), ad)
	c.Assert(err, IsNil)
	c.Assert(ad.Rejected(), Equals, true)

	insp, ok := ad.ResponseInspection[ins.ID()]
	c.Assert(ok, Equals, true)
	c.Assert(insp.Opinion, Equals, opinions.Rejected)
	c.Check(insp.Reason, Equals, "sha256 missing for item")
	c.Check(insp.Annotations, DeepEquals, Annotation{
		"product-item-path":   "jammy/20250621/no-image-found.img",
		"product-item-sha256": "2d06e9092ec19fbe3c04402a0c1a53ae7c0b1079b041e5b5988c8febaabe25d3",
	})
}

func (s *simpleStreamDownloadSuite) TestSimpleDownloadInspectorInspectImageSha256Mismatch(c *C) {
	ins := lxd.NewSimpleStreamsDownloadInspector()

	// First process a download JSON to populate the images map
	a := metadata.NewArtifact()
	a.MimeType = mimetype.Lookup("application/json")
	a.CurrentDownload = metadata.Download{URL: "https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/com.ubuntu.cloud:daily:download.json"}

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)

	f, err := files.OpenArtifactFile("testdata/download.json")
	c.Assert(err, IsNil)
	defer f.Close()

	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	// Now test image with wrong SHA256
	ad := metadata.NewArtifact()
	ad.CurrentDownload = metadata.Download{
		URL: "https://cloud-images.ubuntu.com:443/buildd/daily/jammy/20250621/jammy-server-cloudimg-amd64-lxd_combined.tar.gz",
	}
	wrongSha256, _ := digests.NewSha256Digest("1111111111111111111111111111111111111111111111111111111111111111")
	ad.Metadata.Sha256 = wrongSha256
	ad.MimeType = mimetype.Lookup("application/gzip")

	err = ins.InspectRequest(ad)
	c.Assert(err, IsNil)
	c.Assert(ad.RequestPending(), Equals, true)

	err = ins.InspectArtifact(strings.NewReader("not json"), ad)
	c.Assert(err, IsNil)
	c.Assert(ad.Rejected(), Equals, true)

	insp, ok := ad.ResponseInspection[ins.ID()]
	c.Assert(ok, Equals, true)
	c.Assert(insp.Opinion, Equals, opinions.Rejected)
	c.Assert(insp.Reason, Equals, "sha256 mismatch")
	c.Check(insp.Annotations["expected-sha256"], Equals, "b30a183187a391e87c7752a4e52724f6cc66ddcc875f8e842d1937680f243d8c")
	c.Check(insp.Annotations["product-item-sha256"], Equals, "1111111111111111111111111111111111111111111111111111111111111111")
	c.Check(insp.Annotations, DeepEquals, Annotation{
		"expected-sha256":     "b30a183187a391e87c7752a4e52724f6cc66ddcc875f8e842d1937680f243d8c",
		"product-item-sha256": "1111111111111111111111111111111111111111111111111111111111111111",
	})
}
