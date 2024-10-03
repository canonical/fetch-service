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

package config_test

import (
	"net/url"
	"testing"

	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/inspectors/craft/config"
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

func getTestSourcecraftConfig() config.SourcecraftInspectorConfig {
	return config.SourcecraftInspectorConfig{
		Origins: []glob.Glob{
			glob.MustCompile("https://github.com:443"),
		},
	}
}

func (t *configSuite) TestSourcecraftUrlInfo(c *C) {
	for _, tc := range []struct {
		url     string
		project string
		msg     string
	}{
		{"https://github.com/canonical/fetch-service/git-upload-pack", "fetch-service", ""},
		{"https://github.com:443/canonical/fetch-service/git-upload-pack", "fetch-service", ""},
		{"http://github.com/canonical/fetch-service/git-upload-pack", "", "invalid origin http://github.com"},
		{"https://github.com/canonical/fetch-service", "", "not a valid git upload-pack path"},
		{"https://github.com/canonical/fetch-service/info?service=git-upload-pack", "", "not a valid git upload-pack path"},
	} {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestSourcecraftConfig()
		info, err := config.NewSourcecraftUrlInfo(u, &cfg)

		if tc.msg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.SourcecraftUrlInfo{
				Project: tc.project,
			})
		} else {
			c.Assert(err, ErrorMatches, tc.msg)
		}
	}
}
