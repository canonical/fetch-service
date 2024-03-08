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

	"github.com/canonical/fetch-service/metadata"
)

func (t *metadataSuite) TestOpinionKindMarshal(c *C) {
	var kind struct {
		K metadata.OpinionKind `json:"k"`
	}

	for _, tc := range []struct {
		kind metadata.OpinionKind
		res  string
	}{
		{metadata.Approved, "Approved"},
		{metadata.Rejected, "Rejected"},
		{metadata.Unknown, "Unknown"},
	} {

		kind.K = tc.kind

		data, err := json.Marshal(kind)
		c.Assert(err, IsNil)
		c.Assert(string(data), Equals, fmt.Sprintf(`{"k":"%s"}`, tc.res))
	}
}

func (t *metadataSuite) TestOpinionKindUnmarshal(c *C) {
	var kind struct {
		K metadata.OpinionKind `json:"k"`
	}

	for _, tc := range []struct {
		kind string
		res  metadata.OpinionKind
	}{
		{"Approved", metadata.Approved},
		{"Rejected", metadata.Rejected},
		{"Unknown", metadata.Unknown},
	} {
		data := []byte(fmt.Sprintf(`{"k":"%s"}`, tc.kind))

		err := json.Unmarshal(data, &kind)
		c.Assert(err, IsNil)
		c.Assert(kind.K, Equals, tc.res)
	}
}

func (t *metadataSuite) TestRequestOpinions(c *C) {
	for _, tc := range []struct {
		opinions []metadata.OpinionKind
		rejected bool
		pending  bool
	}{
		{[]metadata.OpinionKind{}, true, false},
		{[]metadata.OpinionKind{metadata.Unknown}, true, false},
		{[]metadata.OpinionKind{metadata.Rejected}, true, false},
		{[]metadata.OpinionKind{metadata.Pending}, false, true},
		{[]metadata.OpinionKind{metadata.Unknown, metadata.Unknown, metadata.Unknown}, true, false},
		{[]metadata.OpinionKind{metadata.Unknown, metadata.Pending, metadata.Unknown}, false, true},
		{[]metadata.OpinionKind{metadata.Unknown, metadata.Unknown, metadata.Rejected}, true, false},
		{[]metadata.OpinionKind{metadata.Pending, metadata.Rejected, metadata.Unknown}, true, false},
	} {
		a := metadata.NewArtefact()
		a.State = metadata.RequestState

		for i, o := range tc.opinions {
			id := fmt.Sprintf("insp%d", i)
			a.RequestInspection[id] = &metadata.Inspection{Opinion: o}
		}

		c.Assert(a.Approved(), Equals, false)
		c.Assert(a.Rejected(), Equals, tc.rejected)
		c.Assert(a.Pending(), Equals, tc.pending)
	}
}

func (t *metadataSuite) TestResponseOpinions(c *C) {
	for _, tc := range []struct {
		opinions []metadata.OpinionKind
		rejected bool
		approved bool
	}{
		{[]metadata.OpinionKind{}, true, false},
		{[]metadata.OpinionKind{metadata.Unknown}, true, false},
		{[]metadata.OpinionKind{metadata.Rejected}, true, false},
		{[]metadata.OpinionKind{metadata.Approved}, false, true},
		{[]metadata.OpinionKind{metadata.Unknown, metadata.Unknown, metadata.Unknown}, true, false},
		{[]metadata.OpinionKind{metadata.Unknown, metadata.Approved, metadata.Unknown}, false, true},
		{[]metadata.OpinionKind{metadata.Unknown, metadata.Unknown, metadata.Rejected}, true, false},
		{[]metadata.OpinionKind{metadata.Approved, metadata.Rejected, metadata.Unknown}, true, false},
	} {
		a := metadata.NewArtefact()
		a.State = metadata.ResponseState

		for i, o := range tc.opinions {
			id := fmt.Sprintf("insp%d", i)
			a.ResponseInspection[id] = &metadata.Inspection{Opinion: o}
		}

		c.Assert(a.Pending(), Equals, false)
		c.Assert(a.Rejected(), Equals, tc.rejected)
		c.Assert(a.Approved(), Equals, tc.approved)
	}
}
