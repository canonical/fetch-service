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

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/inspectors/apt/config"
)

type configSuite struct{}

var _ = Suite(&configSuite{})

func Test(t *testing.T) { TestingT(t) }

func getTestAptConfig() config.AptInspectorConfig {
	return config.AptInspectorConfig{
		Repositories: map[string]config.AptInspectorConfigRepository{
			"default": {
				Urls: []glob.Glob{
					glob.MustCompile("http://*.ubuntu.com/ubuntu"),
					glob.MustCompile("https://*.ubuntu.com:443/**/ubuntu"),
				},
				Dists:      []glob.Glob{glob.MustCompile("focal")},
				Components: []glob.Glob{glob.MustCompile("main")},
				PublicKey:  "",
			},
		},
	}
}

type inReleaseUrlInfoTest struct {
	url      string
	msg      string
	expected *config.InReleaseUrlInfo
}

var inReleaseUrlInfoTests = []inReleaseUrlInfoTest{{
	url: "http://archive.ubuntu.com/ubuntu/dists/focal/InRelease",
	expected: &config.InReleaseUrlInfo{
		CfgName:    "default",
		Origin:     "http://archive.ubuntu.com",
		Repository: "http://archive.ubuntu.com/ubuntu",
		Dist:       "focal",
	},
}, {
	url: "http://us.archive.ubuntu.com/ubuntu/dists/focal/InRelease",
	expected: &config.InReleaseUrlInfo{
		CfgName:    "default",
		Origin:     "http://us.archive.ubuntu.com",
		Repository: "http://us.archive.ubuntu.com/ubuntu",
		Dist:       "focal",
	},
}, {
	url: "https://esm.ubuntu.com/fips/ubuntu/dists/focal/InRelease",
	expected: &config.InReleaseUrlInfo{
		CfgName:    "default",
		Origin:     "https://esm.ubuntu.com:443",
		Repository: "https://esm.ubuntu.com:443/fips/ubuntu",
		Dist:       "focal",
	},
}, {
	url: "http://archive.ubuntu.com/ubuntu/dists/jammy/InRelease",
	msg: "invalid dist: jammy",
}, {
	url: "http://archive.ubuntu.com/ubuntu/dists/focal/NotInRelease",
	msg: "invalid InRelease URL path: .*",
}, {
	url: "http://archive.ubuntu.com/ubuntu/focal/InRelease",
	msg: "invalid repository URL: http://.*",
}}

func (t *configSuite) TestInReleaseUrlInfo(c *C) {
	for _, tc := range inReleaseUrlInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewInReleaseUrlInfo(u, &cfg)

		if tc.msg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, tc.expected)
		} else {
			c.Assert(err, ErrorMatches, tc.msg)
		}
	}
}

type packagesUrlInfoTest struct {
	url      string
	msg      string
	expected *config.PackagesUrlInfo
}

var packagesUrlInfoTests = []packagesUrlInfoTest{{
	url: "http://archive.ubuntu.com/ubuntu/dists/focal/main/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	expected: &config.PackagesUrlInfo{
		CfgName:      "default",
		Origin:       "http://archive.ubuntu.com",
		Repository:   "http://archive.ubuntu.com/ubuntu",
		Dist:         "focal",
		Component:    "main",
		Architecture: "amd64",
		Digest:       "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	},
}, {
	url: "https://esm.ubuntu.com/fips/ubuntu/dists/focal/main/binary-amd64/Packages.gz",
	expected: &config.PackagesUrlInfo{
		CfgName:      "default",
		Origin:       "https://esm.ubuntu.com:443",
		Repository:   "https://esm.ubuntu.com:443/fips/ubuntu",
		Dist:         "focal",
		Component:    "main",
		Architecture: "amd64",
	},
}, {
	url: "http://archive.not-ubuntu.com/ubuntu/dists/focal/main/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	msg: "invalid repository: .*",
}, {
	url: "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	msg: "invalid dist: jammy",
}, {
	url: "http://archive.ubuntu.com/ubuntu/dists/focal/universe/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	msg: "invalid component: universe",
}}

func (t *configSuite) TestPackagesUrlInfo(c *C) {
	for _, tc := range packagesUrlInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewPackagesUrlInfo(u, &cfg)

		if tc.msg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, tc.expected)
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
