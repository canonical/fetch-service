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

package utils_test

import (
	"strings"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/utils"
)

type jsonUtilsSuite struct{}

var _ = Suite(&jsonUtilsSuite{})

func (t *jsonUtilsSuite) TestMarshalNoEscapeHTML(c *C) {
	type jsonTest struct {
		Value string `json:"value"`
	}

	for _, tc := range []struct {
		value  string
		result string
	}{
		{"", `{"value":""}`},
		{"foo", `{"value":"foo"}`},
		{"föó <foo@example.com>", `{"value":"föó <foo@example.com>"}`},
	} {
		d := jsonTest{Value: tc.value}
		j, err := utils.JSONMarshalNoHTMLEscape(d)
		c.Assert(err, IsNil)
		c.Check(strings.TrimSpace(string(j)), Equals, tc.result)
	}
}

func (t *jsonUtilsSuite) TestMarshalIndentNoEscapeHTML(c *C) {
	type jsonTest struct {
		Value string `json:"value"`
	}

	for _, tc := range []struct {
		value  string
		result string
	}{
		{"", "{\n!-\"value\": \"\"\n!}"},
		{"foo", "{\n!-\"value\": \"foo\"\n!}"},
		{"föó <foo@example.com>", "{\n!-\"value\": \"föó <foo@example.com>\"\n!}"},
	} {
		d := jsonTest{Value: tc.value}
		j, err := utils.JSONMarshalIndentNoHTMLEscape(d, "!", "-")
		c.Assert(err, IsNil)
		c.Check(strings.TrimSpace(string(j)), Equals, tc.result)
	}
}
