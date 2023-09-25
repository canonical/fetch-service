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
