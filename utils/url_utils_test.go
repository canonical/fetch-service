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
	"net/url"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/utils"
)

type urlUtilsSuite struct{}

var _ = Suite(&urlUtilsSuite{})

func (t *urlUtilsSuite) TestNormalizedOrigin(c *C) {
	for _, tc := range []struct {
		url    string
		origin string
	}{
		{"http://foo.org/bla", "http://foo.org"},
		{"http://foo.org:8888/bla", "http://foo.org:8888"},
		{"https://foo.org/bla", "https://foo.org:443"},
		{"https://foo.org:8888/bla", "https://foo.org:8888"},
	} {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		o := utils.NormalizedOrigin(u)
		c.Check(o, Equals, tc.origin)
	}
}

func (t *urlUtilsSuite) TestNormalizedURL(c *C) {
	for _, tc := range []struct {
		url        string
		normalized string
	}{
		{"Http://foo.org/bla", "http://foo.org/bla"},
		{"http://foo.org/bla/", "http://foo.org/bla/"},
		{"https://foo.org//bla", "https://foo.org/bla"},
		{"https://foo.org:8888/a/b/../c", "https://foo.org:8888/a/c"},
	} {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil, Commentf("test case: %+v", tc))
		c.Check(utils.NormalizedURL(u), Equals, tc.normalized, Commentf("test case: %+v", tc))
	}
}
