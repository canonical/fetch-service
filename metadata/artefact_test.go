// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023 Canonical Ltd.
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
	"encoding/json"
	"fmt"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

func (t *metadataSuite) TestOpinionKindMarshal(c *C) {
	var kind struct {
		K opinions.OpinionKind `json:"k"`
	}

	for _, tc := range []struct {
		kind opinions.OpinionKind
		res  string
	}{
		{opinions.Approved, "Approved"},
		{opinions.Rejected, "Rejected"},
		{opinions.Unknown, "Unknown"},
	} {

		kind.K = tc.kind

		data, err := json.Marshal(kind)
		c.Assert(err, IsNil)
		c.Assert(string(data), Equals, fmt.Sprintf(`{"k":"%s"}`, tc.res))
	}
}

func (t *metadataSuite) TestOpinionKindUnmarshal(c *C) {
	var kind struct {
		K opinions.OpinionKind `json:"k"`
	}

	for _, tc := range []struct {
		kind string
		res  opinions.OpinionKind
	}{
		{"Approved", opinions.Approved},
		{"Rejected", opinions.Rejected},
		{"Unknown", opinions.Unknown},
	} {
		data := []byte(fmt.Sprintf(`{"k":"%s"}`, tc.kind))

		err := json.Unmarshal(data, &kind)
		c.Assert(err, IsNil)
		c.Assert(kind.K, Equals, tc.res)
	}
}

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
			a.RequestInspection[id] = &metadata.Inspection{Opinion: o}
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
			a.ResponseInspection[id] = &metadata.Inspection{Opinion: o}
		}

		c.Assert(a.Rejected(), Equals, tc.rejected)
		c.Assert(a.Approved(), Equals, tc.approved)
	}
}

func (t *metadataSuite) TestSetRequestOpinion(c *C) {
	for _, tc := range []struct {
		op          opinions.OpinionKind
		effectiveOp opinions.OpinionKind
	}{
		{opinions.Unknown, opinions.Unknown},
		{opinions.Pending, opinions.Pending},
		{opinions.Rejected, opinions.Rejected},
		{opinions.Approved, opinions.Rejected}, // can't approve during request inspection
	} {
		a := metadata.NewArtefact()
		a.SetRequestOpinion(inspectors.DefaultInspector{}.ID(), tc.op, "testing %d", 1).Annotate(
			metadata.Annotation{"foo": "bar"},
		)
		c.Assert(*a.RequestInspection["default"], DeepEquals, metadata.Inspection{
			Opinion:     tc.effectiveOp,
			Reason:      "testing 1",
			Annotations: metadata.Annotation{"foo": "bar"},
		})
	}
}

func (t *metadataSuite) TestSetResponseOpinion(c *C) {
	for _, tc := range []struct {
		op          opinions.OpinionKind
		effectiveOp opinions.OpinionKind
	}{
		{opinions.Unknown, opinions.Unknown},
		{opinions.Pending, opinions.Rejected}, // can't set opinion to pending in response inspection
		{opinions.Rejected, opinions.Rejected},
		{opinions.Approved, opinions.Approved},
	} {
		a := metadata.NewArtefact()
		a.SetResponseOpinion(inspectors.DefaultInspector{}.ID(), tc.op, "testing %d", 1).Annotate(
			metadata.Annotation{"foo": "bar"},
		)
		c.Assert(*a.ResponseInspection["default"], DeepEquals, metadata.Inspection{
			Opinion:     tc.effectiveOp,
			Reason:      "testing 1",
			Annotations: metadata.Annotation{"foo": "bar"},
		})
	}
}
