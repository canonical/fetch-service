// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2024 Canonical Ltd.
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

	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/metadata"
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
		a := metadata.NewArtefact()

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
		a := metadata.NewArtefact()

		for i, o := range tc.opinions {
			id := fmt.Sprintf("insp%d", i)
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

func (ins *testInspector) InspectRequest(a RequestArtefact) error {
	return nil
}

func (ins *testInspector) InspectArtefact(f ArtefactReader, a ResponseArtefact) error {
	return nil
}

func (t *metadataSuite) TestSetRequestPending(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtefact()
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
	a := metadata.NewArtefact()
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
	a := metadata.NewArtefact()
	a.SetRequestUnknown(ins, "testing %d", 1).Annotate(
		Annotation{"foo": "bar"},
	)
	c.Assert(*a.RequestInspection["test-inspector"], DeepEquals, Inspection{
		Opinion:     opinions.Unknown,
		Reason:      "testing 1",
		Annotations: Annotation{"foo": "bar"},
	})
}

func (t *metadataSuite) TestSetResponseApproved(c *C) {
	ins := &testInspector{}
	a := metadata.NewArtefact()
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
	a := metadata.NewArtefact()
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
	a := metadata.NewArtefact()
	a.SetResponseUnknown(ins, "testing %d", 1).Annotate(
		Annotation{"foo": "bar"},
	)
	c.Assert(*a.ResponseInspection["test-inspector"], DeepEquals, Inspection{
		Opinion:     opinions.Unknown,
		Reason:      "testing 1",
		Annotations: Annotation{"foo": "bar"},
	})
}
