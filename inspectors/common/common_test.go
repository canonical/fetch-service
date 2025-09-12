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

package common_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/common"
)

func Test(t *testing.T) { TestingT(t) }

type commonSuite struct {
}

func (t *commonSuite) SetUpTest(c *C) {
}

func (t *commonSuite) TearDownTest(c *C) {
}

var _ = Suite(&commonSuite{})

func (t *commonSuite) TestAnnotationAdd(c *C) {
	notes := common.Annotation{}
	notes.Add("foo", "bar")
	notes.Add("num", 42)
	c.Assert(notes, DeepEquals, common.Annotation{
		"foo": "bar",
		"num": 42,
	})
}

func (t *commonSuite) TestAnnotationAppend(c *C) {
	notes := common.Annotation{"foo": "bar"}
	moreNotes := common.Annotation{"num": 42}
	notes.Append(moreNotes)
	c.Assert(notes, DeepEquals, common.Annotation{
		"foo": "bar",
		"num": 42,
	})
}

func (t *commonSuite) TestInspectionAnnotate(c *C) {
	insp := common.Inspection{}
	insp.Annotate(common.Annotation{"foo": "bar"})
	c.Assert(insp.Annotations, DeepEquals, common.Annotation{"foo": "bar"})

	insp.Annotate(common.Annotation{"num": 42})
	c.Assert(insp.Annotations, DeepEquals, common.Annotation{
		"foo": "bar",
		"num": 42,
	})
}
