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

package glob_test

import (
	"encoding/json"
	"testing"

	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"

	"github.com/canonical/fetch-service/glob"
)

type configSuite struct{}

var _ = Suite(&configSuite{})

func Test(t *testing.T) { TestingT(t) }

func (t *configSuite) TestGlobUnmarshal(c *C) {
	type testGlob struct {
		Foo glob.Glob `yaml:"foo"`
	}

	data := []byte(`foo: "*.txt"`)

	var y testGlob
	err := yaml.Unmarshal(data, &y)
	c.Assert(err, IsNil)
	c.Assert(y.Foo, DeepEquals, glob.MustCompile("*.txt"))
}

func (t *configSuite) TestGlobUnmarshalJSON(c *C) {
	type testGlob struct {
		Foo glob.Glob `json:"foo"`
	}

	data := []byte(`{"foo": "*.txt"}`)

	var j testGlob
	err := json.Unmarshal(data, &j)
	c.Assert(err, IsNil)
	c.Assert(j.Foo, DeepEquals, glob.MustCompile("*.txt"))
}

func (t *configSuite) TestGlobMatch(c *C) {
	for _, tc := range []struct {
		pattern string
		s       string
		matches bool
	}{
		{"b*n*a", "banana", true},
		{"[Aa]p*le", "Apple", true},
		{"[Aa]p*le", "Pineapple", false},
	} {
		g := glob.MustCompile(tc.pattern)
		c.Check(g.Match(tc.s), Equals, tc.matches)
	}
}
