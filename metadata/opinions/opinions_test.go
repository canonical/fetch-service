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

package opinions_test

import (
	"encoding/json"
	"fmt"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/metadata/opinions"
)

func Test(t *testing.T) { TestingT(t) }

type opinionsSuite struct{}

var _ = Suite(&opinionsSuite{})

func (s *opinionsSuite) TestOpinionsValues(c *C) {
	c.Assert(opinions.Unknown, Equals, opinions.OpinionKind(0))
	c.Assert(opinions.Rejected, Equals, opinions.OpinionKind(1))
	c.Assert(opinions.Approved, Equals, opinions.OpinionKind(2))
	c.Assert(opinions.Pending, Equals, opinions.OpinionKind(3))
}

func (s *opinionsSuite) TestOpinionsMarshal(c *C) {
	type Foo struct {
		Op opinions.OpinionKind `json:"op"`
	}

	for _, tc := range []struct {
		op     opinions.OpinionKind
		result string
	}{
		{opinions.Unknown, "Unknown"},
		{opinions.Rejected, "Rejected"},
		{opinions.Approved, "Approved"},
		{opinions.Pending, "Pending"},
	} {
		j, err := json.Marshal(Foo{tc.op})
		c.Assert(err, IsNil)
		c.Check(j, DeepEquals, []byte(fmt.Sprintf(`{"op":"%s"}`, tc.result)))
	}
}

func (s *opinionsSuite) TestSha1DigestUnmarshal(c *C) {
	type Foo struct {
		Op opinions.OpinionKind `json:"op"`
	}

	for _, tc := range []struct {
		opstr  string
		result opinions.OpinionKind
	}{
		{"Unknown", opinions.Unknown},
		{"Rejected", opinions.Rejected},
		{"Approved", opinions.Approved},
		{"Pending", opinions.Pending},
	} {
		var foo Foo
		err := json.Unmarshal([]byte(fmt.Sprintf(`{"op":"%s"}`, tc.opstr)), &foo)
		c.Assert(err, IsNil)
		c.Check(foo.Op, Equals, tc.result)
	}
}
