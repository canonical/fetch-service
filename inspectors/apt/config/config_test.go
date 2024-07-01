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

	"github.com/gobwas/glob"
	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"

	"github.com/canonical/fetch-service/inspectors/apt/config"
)

type configSuite struct{}

var _ = Suite(&configSuite{})

func Test(t *testing.T) { TestingT(t) }

func (t *configSuite) TestGlobUnmarshal(c *C) {
	type testGlob struct {
		Foo config.Glob `yaml:"foo"`
	}

	data := []byte(`foo: "*.txt"`)

	var y testGlob
	err := yaml.Unmarshal(data, &y)
	c.Assert(err, IsNil)
	c.Assert(y.Foo.G, DeepEquals, glob.MustCompile("*.txt"))
}

func getTestAptConfig() config.AptInspectorConfig {
	return config.AptInspectorConfig{
		Repositories: map[string]config.AptInspectorConfigRepository{
			"default": {
				Urls:       []config.Glob{{G: glob.MustCompile("http://*.ubuntu.com/ubuntu")}},
				Dists:      []config.Glob{{G: glob.MustCompile("focal")}},
				Components: []config.Glob{{G: glob.MustCompile("main")}},
				PublicKey:  "",
			},
		},
	}
}

func (t *configSuite) TestInReleaseUrlInfo(c *C) {
	for _, tc := range []struct {
		url string
		msg string
	}{
		{"http://archive.ubuntu.com/ubuntu/dists/focal/InRelease", ""},
		{"http://us.archive.ubuntu.com/ubuntu/dists/focal/InRelease", ""},
		{"http://archive.ubuntu.com/ubuntu/dists/jammy/InRelease", "invalid dist: jammy"},
		{"http://archive.ubuntu.com/ubuntu/dists/focal/NotInRelease", "invalid InRelease URL path: .*"},
		{"http://archive.ubuntu.com/ubuntu/focal/InRelease", "invalid repository URL: http://.*"},
	} {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewInReleaseUrlInfo(u, &cfg)

		if tc.msg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.InReleaseUrlInfo{
				CfgName:    "default",
				Origin:     "http://archive.ubuntu.com",
				Repository: "http://archive.ubuntu.com/ubuntu",
				Dist:       "focal",
			})
		} else {
			c.Assert(err, ErrorMatches, tc.msg)
		}
	}
}

func (t *configSuite) TestPackagesUrlInfo(c *C) {
	for _, tc := range []struct {
		url string
		msg string
	}{
		{"http://archive.ubuntu.com/ubuntu/dists/focal/main/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03", ""},
		{"http://archive.not-ubuntu.com/ubuntu/dists/focal/main/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03", "invalid repository: .*"},
		{"http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03", "invalid dist: jammy"},
		{"http://archive.ubuntu.com/ubuntu/dists/focal/universe/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03", "invalid component: universe"},
	} {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewPackagesUrlInfo(u, &cfg)

		if tc.msg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.PackagesUrlInfo{
				CfgName:      "default",
				Origin:       "http://archive.ubuntu.com",
				Repository:   "http://archive.ubuntu.com/ubuntu",
				Dist:         "focal",
				Component:    "main",
				Architecture: "amd64",
				Digest:       "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
			})
		} else {
			c.Assert(err, ErrorMatches, tc.msg)
		}
	}
}

func (t *configSuite) TestTranslationUrlInfo(c *C) {
	for _, tc := range []struct {
		url string
		msg string
	}{
		{"http://archive.ubuntu.com/ubuntu/dists/focal/main/i18n/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03", ""},
		{"http://archive.not-ubuntu.com/ubuntu/dists/focal/main/i18n/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03", "invalid repository: .*"},
		{"http://archive.ubuntu.com/ubuntu/dists/focal/main/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03", "invalid translation URL path: .*"},
		{"http://archive.ubuntu.com/ubuntu/dists/jammy/main/i18n/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03", "invalid dist: jammy"},
		{"http://archive.ubuntu.com/ubuntu/dists/focal/universe/i18n/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03", "invalid component: universe"},
	} {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewTranslationUrlInfo(u, &cfg)

		if tc.msg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.TranslationUrlInfo{
				CfgName:    "default",
				Origin:     "http://archive.ubuntu.com",
				Repository: "http://archive.ubuntu.com/ubuntu",
				Dist:       "focal",
				Component:  "main",
				Digest:     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
			})
		} else {
			c.Assert(err, ErrorMatches, tc.msg)
		}
	}
}

func (t *configSuite) TestDebPackageUrlInfo(c *C) {
	for _, tc := range []struct {
		url string
		msg string
	}{
		{"http://archive.ubuntu.com/ubuntu/pool/main/c/curl/libcurl3-gnutls_7.81.0-1ubuntu1.16_amd64.deb", ""},
		{"http://archive.ubuntu.com/ubuntu/pool/main/c/c/c/curl/libcurl3-gnutls_7.81.0-1ubuntu1.16_amd64.deb", ""},
		{"http://archive.not-ubuntu.com/ubuntu/pool/main/c/curl/libcurl3-gnutls_7.81.0-1ubuntu1.16_amd64.deb", "invalid repository: http://archive.not-ubuntu.com/ubuntu"},
		{"http://archive.ubuntu.com/ubuntu/dists/focal/main/c/curl/libcurl3-gnutls_7.81.0-1ubuntu1.16_amd64.deb", "invalid repository URL: http://.*"},
	} {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewDebPackageUrlInfo(u, &cfg)

		if tc.msg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.DebPackageUrlInfo{
				CfgName:      "default",
				Origin:       "http://archive.ubuntu.com",
				Repository:   "http://archive.ubuntu.com/ubuntu",
				Component:    "main",
				Name:         "libcurl3-gnutls",
				Version:      "7.81.0-1ubuntu1.16",
				Architecture: "amd64",
			})
		} else {
			c.Assert(err, ErrorMatches, tc.msg)
		}
	}
}
