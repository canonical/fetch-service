// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2025 Canonical Ltd.
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

package metadata_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
)

func (t *metadataSuite) TestRequestOpinions(c *C) {
	for _, tc := range []struct {
		opinions []opinions.OpinionKind
		rejected bool
		pending  bool
	}{
		{[]opinions.OpinionKind{}, false, false},
		{[]opinions.OpinionKind{opinions.Unknown}, false, false},
		{[]opinions.OpinionKind{opinions.Rejected}, true, false},
		{[]opinions.OpinionKind{opinions.Pending}, false, true},
		{[]opinions.OpinionKind{opinions.Unknown, opinions.Unknown, opinions.Unknown}, false, false},
		{[]opinions.OpinionKind{opinions.Unknown, opinions.Pending, opinions.Unknown}, false, true},
		{[]opinions.OpinionKind{opinions.Unknown, opinions.Unknown, opinions.Rejected}, true, false},
		{[]opinions.OpinionKind{opinions.Pending, opinions.Rejected, opinions.Unknown}, true, false},
	} {
		a := metadata.NewArtifact()

		for i, o := range tc.opinions {
			id := fmt.Sprintf("insp%d", i)
			a.RequestInspection[id] = &Inspection{Opinion: o}
		}

		c.Assert(a.Approved(), Equals, false)
		c.Assert(a.RequestRejected(), Equals, tc.rejected)
		c.Assert(a.RequestPending(), Equals, tc.pending)
	}
}

func (t *metadataSuite) TestResponseOpinions(c *C) {
	for _, tc := range []struct {
		opinions []opinions.OpinionKind
		rejected bool
		approved bool
	}{
		{[]opinions.OpinionKind{}, true, false},
		{[]opinions.OpinionKind{opinions.Unknown}, true, false},
		{[]opinions.OpinionKind{opinions.Rejected}, true, false},
		{[]opinions.OpinionKind{opinions.Approved}, false, true},
		{[]opinions.OpinionKind{opinions.Unknown, opinions.Unknown, opinions.Unknown}, true, false},
		{[]opinions.OpinionKind{opinions.Unknown, opinions.Approved, opinions.Unknown}, false, true},
		{[]opinions.OpinionKind{opinions.Unknown, opinions.Unknown, opinions.Rejected}, true, false},
		{[]opinions.OpinionKind{opinions.Approved, opinions.Rejected, opinions.Unknown}, true, false},
	} {
		a := metadata.NewArtifact()

		for i, o := range tc.opinions {
			id := fmt.Sprintf("insp%d", i)
			a.RequestInspection[id] = &Inspection{Opinion: opinions.Pending}
			a.ResponseInspection[id] = &Inspection{Opinion: o}
		}

		c.Assert(a.Rejected(), Equals, tc.rejected)
		c.Assert(a.Approved(), Equals, tc.approved)
	}
}

type testInspector struct{}

func (ins testInspector) ID() string {
	return "test-inspector"
}

func (ins *testInspector) InspectRequest(a RequestArtifact) error {
	return nil
}

func (ins *testInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	return nil
}

type testInspector2 struct {
	testInspector
}

func (ins testInspector2) ID() string {
	return "test-inspector2"
}

func (t *metadataSuite) TestNewArtifact(c *C) {
	a := metadata.NewArtifact()
	c.Check(a.MetadataVersion, Equals, "0.3")
	c.Check(a.RequestInspection, Not(IsNil))
	c.Check(a.ResponseInspection, Not(IsNil))
	c.Check(a.Metadata, Not(IsNil))
	c.Check(a.Downloads, Not(IsNil))
	c.Check(a.CurrentDownload, Not(IsNil))
	c.Check(a.Request, IsNil)
}

func (t *metadataSuite) TestRequestHeader(c *C) {
	for _, tc := range []struct {
		headerExists  bool
		expectedValue []string
		expectedOk    bool
	}{
		{true, []string{"bar"}, true},
		{false, nil, false},
	} {
		a := metadata.NewArtifact()
		a.CurrentDownload.RequestHeader = http.Header{}

		if tc.headerExists {
			a.CurrentDownload.RequestHeader["foo"] = []string{"bar"}
		}

		res, ok := a.RequestHeader("foo")
		c.Assert(ok, Equals, tc.expectedOk)
		if ok {
			c.Assert(res, DeepEquals, tc.expectedValue)
		}
	}
}

func (t *metadataSuite) TestRequestHeaderContains(c *C) {
	for _, tc := range []struct {
		entry    string
		haystack []string
		needle   string
		exists   bool
	}{
		{"foo", []string{"foo1", "foo2"}, "foo1", true},
		{"foo", []string{"foo1", "foo2"}, "foo2", true},
		{"foo", []string{"foo1", "foo2"}, "foo3", false},
		{"foo", []string{}, "foo1", false},
	} {
		a := metadata.NewArtifact()
		a.CurrentDownload.RequestHeader = http.Header{}
		a.CurrentDownload.RequestHeader[tc.entry] = tc.haystack

		res := a.RequestHeaderContains("foo", tc.needle)
		c.Assert(res, Equals, tc.exists, Commentf("test case: %+v", tc))

		res = a.RequestHeaderContains("bar", tc.needle)
		c.Assert(res, Equals, false, Commentf("test case: %+v", tc))
	}
}

func (t *metadataSuite) TestContentType(c *C) {
	a := metadata.NewArtifact()
	a.CurrentDownload.ContentType = "application/test"
	c.Assert(a.ContentType(), Equals, "application/test")
}

func (t *metadataSuite) TestDownloadURL(c *C) {
	a := metadata.NewArtifact()
	a.CurrentDownload.URL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	c.Assert(a.DownloadURL(), Equals, "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
}

func (t *metadataSuite) TestHTTPRequest(c *C) {
	a := metadata.NewArtifact()
	c.Assert(a.HTTPRequest(), Equals, a.Request)
}

func (t *metadataSuite) TestSetRequestBody(c *C) {
	a := metadata.NewArtifact()
	a.Request = &http.Request{}
	f := io.NopCloser(strings.NewReader("content"))
	a.SetRequestBody(f)
	c.Assert(a.Request.Body, Equals, f)
}

func (t *metadataSuite) TestSetArtifactMetadata(c *C) {
	for _, tc := range []struct {
		assignedType string
		expectedType string
	}{
		{"application/test", "application/test"},
		{"", "text/plain"},
	} {
		m := ArtifactMetadata{
			Type:          tc.assignedType,
			Name:          "froblator",
			Version:       "3.14.15",
			Vendor:        "Acme",
			Description:   "Too lazy to add one",
			Author:        "J. Random Hacker",
			AuthorEmail:   "root@localhost",
			Architecture:  "z80",
			License:       "CC0",
			Copyright:     "Copyright 1976 Acme Corp.",
			SourcePackage: "my-source-package",
		}

		a := metadata.NewArtifact()
		a.Metadata.Type = "text/plain"
		a.SetArtifactMetadata(m)

		c.Check(a.Metadata.Type, Equals, tc.expectedType)
		c.Check(a.Metadata.Name, Equals, m.Name)
		c.Check(a.Metadata.Version, Equals, m.Version)
		c.Check(a.Metadata.Vendor, Equals, m.Vendor)
		c.Check(a.Metadata.Description, Equals, m.Description)
		c.Check(a.Metadata.Author, Equals, m.Author)
		c.Check(a.Metadata.AuthorEmail, Equals, m.AuthorEmail)
		c.Check(a.Metadata.Architecture, Equals, m.Architecture)
		c.Check(a.Metadata.License, Equals, m.License)
		c.Check(a.Metadata.Copyright, Equals, m.Copyright)
		c.Check(a.Metadata.SourcePackage, Equals, m.SourcePackage)
	}
}

func (t *metadataSuite) TestSize(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Size = 1337
	c.Assert(a.Size(), Equals, int64(1337))
}

func (t *metadataSuite) TestMetadataIs(c *C) {

	for _, tc := range []struct {
		queryType    string
		metadataType string
		mimetype     string
		result       bool
	}{
		{"text/plain", "", "", false},
		{"text/plain", "text/plain", "", true},
		{"text/plain", "", "text/plain", true},
		{"text/plain", "text/plain", "application/octet-stream", true},
		{"text/plain", "application/octet-stream", "text/plain", true},
	} {
		a := metadata.NewArtifact()
		if tc.metadataType != "" {
			a.Metadata.Type = tc.metadataType
		}
		if tc.mimetype != "" {
			a.MimeType = mimetype.Lookup(tc.mimetype)
		}
		res := a.MimetypeIs(tc.queryType)
		c.Check(res, Equals, tc.result)
	}
}

func (t *metadataSuite) TestSha256Digest(c *C) {
	digest := "00e3261a6e0d79c329445acd540fb2b07187a0dcf6017065c8814010283ac67f"
	a := metadata.NewArtifact()
	a.Metadata.Sha256, _ = digests.NewSha256Digest(digest)
	res := a.Sha256()
	c.Assert(res.String(), Equals, digest)
}

func (t *metadataSuite) TestSetRequestPending(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtifact()
	a.SetRequestPending(ins, "testing %d", 1).Annotate(
		Annotation{"foo": "bar"},
	)
	c.Assert(*a.RequestInspection["test-inspector"], DeepEquals, Inspection{
		Opinion:     opinions.Pending,
		Reason:      "testing 1",
		Annotations: Annotation{"foo": "bar"},
	})
}

func (t *metadataSuite) TestSetRequestRejected(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtifact()
	a.SetRequestRejected(ins, "testing %d", 1).Annotate(
		Annotation{"foo": "bar"},
	)
	c.Assert(*a.RequestInspection["test-inspector"], DeepEquals, Inspection{
		Opinion:     opinions.Rejected,
		Reason:      "testing 1",
		Annotations: Annotation{"foo": "bar"},
	})
}

func (t *metadataSuite) TestSetRequestUnknown(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtifact()
	a.SetRequestUnknown(ins, "testing %d", 1).Annotate(
		Annotation{"foo": "bar"},
	)
	c.Assert(*a.RequestInspection["test-inspector"], DeepEquals, Inspection{
		Opinion:     opinions.Unknown,
		Reason:      "testing 1",
		Annotations: Annotation{"foo": "bar"},
	})
}

func (t *metadataSuite) TestRequestRejected(c *C) {
	for _, tc := range []struct {
		reject   bool
		rejected bool
	}{
		{true, true},
		{false, false},
	} {
		ins := &testInspector{}
		a := metadata.NewArtifact()
		if tc.reject {
			a.SetRequestRejected(ins, "test")
		}
		c.Check(a.RequestRejected(), Equals, tc.rejected)

	}
}
func (t *metadataSuite) TestSetResponseApproved(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtifact()
	a.SetResponseApproved(ins, "testing %d", 1).Annotate(
		Annotation{"foo": "bar"},
	)
	c.Assert(*a.ResponseInspection["test-inspector"], DeepEquals, Inspection{
		Opinion:     opinions.Approved,
		Reason:      "testing 1",
		Annotations: Annotation{"foo": "bar"},
	})
}

func (t *metadataSuite) TestSetResponseRejected(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtifact()
	a.SetResponseRejected(ins, "testing %d", 1).Annotate(
		Annotation{"foo": "bar"},
	)
	c.Assert(*a.ResponseInspection["test-inspector"], DeepEquals, Inspection{
		Opinion:     opinions.Rejected,
		Reason:      "testing 1",
		Annotations: Annotation{"foo": "bar"},
	})
}

func (t *metadataSuite) TestSetResponseUnknown(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtifact()
	a.SetResponseUnknown(ins, "testing %d", 1).Annotate(
		Annotation{"foo": "bar"},
	)
	c.Assert(*a.ResponseInspection["test-inspector"], DeepEquals, Inspection{
		Opinion:     opinions.Unknown,
		Reason:      "testing 1",
		Annotations: Annotation{"foo": "bar"},
	})
}

func (t *metadataSuite) TestResponseRejected(c *C) {
	ins := &testInspector{}
	ins2 := &testInspector2{}

	for _, tc := range []struct {
		reject   bool
		approve  bool
		rejected bool
	}{
		{true, true, true},
		{true, false, true},
		{false, true, false},
		{false, false, false},
	} {
		a := metadata.NewArtifact()
		if tc.reject {
			a.SetResponseRejected(ins, "test")
		}
		if tc.approve {
			a.SetResponseApproved(ins2, "test")
		}
		c.Check(a.ResponseRejected(), Equals, tc.rejected)
	}
}

func (t *metadataSuite) TestRequestPending(c *C) {
	ins := &testInspector{}
	ins2 := &testInspector2{}

	for _, tc := range []struct {
		reject     bool
		setPending bool
		pending    bool
	}{
		{true, true, false},
		{true, false, false},
		{false, true, true},
		{false, false, false},
	} {
		a := metadata.NewArtifact()
		if tc.reject {
			a.SetResponseRejected(ins, "test")
		}
		if tc.setPending {
			a.SetResponseApproved(ins2, "test")
		}
		c.Check(a.ResponseApproved(), Equals, tc.pending)
	}
}

func (t *metadataSuite) TestResponseApproved(c *C) {
	ins := &testInspector{}
	ins2 := &testInspector2{}

	for _, tc := range []struct {
		reject   bool
		approve  bool
		approved bool
	}{
		{true, true, false},
		{true, false, false},
		{false, true, true},
		{false, false, false},
	} {
		a := metadata.NewArtifact()
		if tc.reject {
			a.SetResponseRejected(ins, "test")
		}
		if tc.approve {
			a.SetResponseApproved(ins2, "test")
		}
		c.Check(a.ResponseApproved(), Equals, tc.approved)
	}
}

func (t *metadataSuite) TestRequestAnnotation(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtifact()
	a.SetRequestUnknown(ins, "test").Annotate(
		Annotation{
			"key": "value",
		},
	)

	_, ok := a.RequestAnnotation("other", "foo")
	c.Check(ok, Equals, false)

	_, ok = a.RequestAnnotation("test-inspector", "foo")
	c.Check(ok, Equals, false)

	res, ok := a.RequestAnnotation("test-inspector", "key")
	c.Check(ok, Equals, true)
	c.Check(res, Equals, "value")
}

func (t *metadataSuite) TestRequestStringAnnotation(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtifact()
	a.SetRequestUnknown(ins, "test").Annotate(
		Annotation{
			"key": "value",
			"int": 123,
		},
	)

	_, ok := a.RequestStringAnnotation("other", "foo")
	c.Check(ok, Equals, false)

	_, ok = a.RequestStringAnnotation("test-inspector", "foo")
	c.Check(ok, Equals, false)

	_, ok = a.RequestStringAnnotation("test-inspector", "int")
	c.Check(ok, Equals, false)

	res, ok := a.RequestStringAnnotation("test-inspector", "key")
	c.Check(ok, Equals, true)
	c.Check(res, Equals, "value")
}

func (t *metadataSuite) TestRequestBoolAnnotation(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtifact()
	a.SetRequestUnknown(ins, "test").Annotate(
		Annotation{
			"key": true,
			"int": 123,
		},
	)

	_, ok := a.RequestBoolAnnotation("other", "foo")
	c.Check(ok, Equals, false)

	_, ok = a.RequestBoolAnnotation("test-inspector", "foo")
	c.Check(ok, Equals, false)

	_, ok = a.RequestBoolAnnotation("test-inspector", "int")
	c.Check(ok, Equals, false)

	res, ok := a.RequestBoolAnnotation("test-inspector", "key")
	c.Check(ok, Equals, true)
	c.Check(res, Equals, true)
}

func (t *metadataSuite) TestResponseAnnotation(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtifact()
	a.SetResponseUnknown(ins, "test").Annotate(
		Annotation{
			"key": "value",
		},
	)

	_, ok := a.ResponseAnnotation("other", "foo")
	c.Check(ok, Equals, false)

	_, ok = a.ResponseAnnotation("test-inspector", "foo")
	c.Check(ok, Equals, false)

	res, ok := a.ResponseAnnotation("test-inspector", "key")
	c.Check(ok, Equals, true)
	c.Check(res, Equals, "value")
}

func (t *metadataSuite) TestResponseStringAnnotation(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtifact()
	a.SetResponseUnknown(ins, "test").Annotate(
		Annotation{
			"key": "value",
			"int": 123,
		},
	)

	_, ok := a.ResponseStringAnnotation("other", "foo")
	c.Check(ok, Equals, false)

	_, ok = a.ResponseStringAnnotation("test-inspector", "foo")
	c.Check(ok, Equals, false)

	_, ok = a.ResponseStringAnnotation("test-inspector", "int")
	c.Check(ok, Equals, false)

	res, ok := a.ResponseStringAnnotation("test-inspector", "key")
	c.Check(ok, Equals, true)
	c.Check(res, Equals, "value")
}

func (t *metadataSuite) TestResponseBoolAnnotation(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtifact()
	a.SetResponseUnknown(ins, "test").Annotate(
		Annotation{
			"key": true,
			"int": 123,
		},
	)

	_, ok := a.ResponseBoolAnnotation("other", "foo")
	c.Check(ok, Equals, false)

	_, ok = a.ResponseBoolAnnotation("test-inspector", "foo")
	c.Check(ok, Equals, false)

	_, ok = a.ResponseBoolAnnotation("test-inspector", "int")
	c.Check(ok, Equals, false)

	res, ok := a.ResponseBoolAnnotation("test-inspector", "key")
	c.Check(ok, Equals, true)
	c.Check(res, Equals, true)
}

func (t *metadataSuite) TestApproved(c *C) {
	for _, tc := range []struct {
		rejectRequest     bool
		setRequestPending bool
		rejectResponse    bool
		approveResponse   bool
		resultApproved    bool
	}{
		{false, false, false, false, false},
		{false, false, false, true, false},
		{true, false, false, true, false},
		{false, true, false, false, false},
		{false, true, true, true, false},
		{false, true, false, true, true},
	} {
		a := metadata.NewArtifact()
		ins := &testInspector{}
		ins2 := &testInspector2{}

		if tc.rejectRequest {
			a.SetRequestRejected(ins, "test")
		}
		if tc.setRequestPending {
			a.SetRequestPending(ins2, "test")
		}
		if tc.rejectResponse {
			a.SetResponseRejected(ins, "test")
		}
		if tc.approveResponse {
			a.SetResponseApproved(ins2, "test")
		}

		c.Check(a.Approved(), Equals, tc.resultApproved)
		c.Check(a.Rejected(), Equals, !tc.resultApproved)
	}
}
