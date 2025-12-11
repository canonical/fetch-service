// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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
	"fmt"
	"net/url"
	"strings"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/inspectors/apt/config"
	"github.com/canonical/fetch-service/logger"
)

type configSuite struct {
	slog logger.Logger
}

var _ = Suite(&configSuite{logger.NewSessionLogger("test")})

func Test(t *testing.T) { TestingT(t) }

func getTestAptConfig() config.AptInspectorConfig {
	return config.AptInspectorConfig{
		Repositories: map[string]config.AptInspectorConfigRepository{
			"default": {
				URLs:       []glob.Glob{glob.MustCompile("http://*.ubuntu.com/ubuntu")},
				Suites:     []glob.Glob{glob.MustCompile("focal")},
				Components: []glob.Glob{glob.MustCompile("main")},
				PublicKey:  "",
			},
			"esm": {
				URLs:       []glob.Glob{glob.MustCompile("https://esm.ubuntu.com:443/fips*/ubuntu")},
				Suites:     []glob.Glob{glob.MustCompile("noble")},
				Components: []glob.Glob{glob.MustCompile("main")},
				PublicKey:  "",
			},
		},
	}
}

type inReleaseURLInfoTest struct {
	url      string // The request URL
	conf     string // The repository configuration entry
	repo     string // The repository name (URL scheme and origin)
	suite    string // The distribution series
	errorMsg string // The error message, if any
}

var inReleaseURLInfoTests = []inReleaseURLInfoTest{{
	url:      "http://archive.ubuntu.com/ubuntu/dists/focal/InRelease",
	conf:     "default",
	repo:     "http://archive.ubuntu.com/ubuntu",
	suite:    "focal",
	errorMsg: "",
}, {
	url:      "http://us.archive.ubuntu.com/ubuntu/dists/focal/InRelease",
	conf:     "default",
	repo:     "http://us.archive.ubuntu.com/ubuntu",
	suite:    "focal",
	errorMsg: "",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/dists/jammy/InRelease",
	conf:     "none",
	repo:     "http://archive.ubuntu.com/ubuntu",
	suite:    "jammy",
	errorMsg: "invalid series: jammy",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/dists/focal/NotInRelease",
	conf:     "none",
	repo:     "http://archive.ubuntu.com/ubuntu",
	suite:    "focal",
	errorMsg: "invalid InRelease URL path: .*",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/focal/InRelease",
	conf:     "none",
	repo:     "none",
	suite:    "http://archive.ubuntu.com/ubuntu",
	errorMsg: "invalid repository URL: http://.*",
}, {
	url:      "https://esm.ubuntu.com:443/fips-preview/ubuntu/dists/noble/InRelease",
	conf:     "esm",
	repo:     "https://esm.ubuntu.com:443/fips-preview/ubuntu",
	suite:    "noble",
	errorMsg: "",
}, {
	url:      "https://esm.ubuntu.com:443/other-repo/ubuntu/dists/noble/InRelease",
	conf:     "none",
	repo:     "https://esm.ubuntu.com:443/other-repo/ubuntu",
	suite:    "noble",
	errorMsg: "invalid repository: https://esm.ubuntu.com:443/other-repo/ubuntu",
}}

func (t *configSuite) TestInReleaseURLInfo(c *C) {
	for _, tc := range inReleaseURLInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewInReleaseURLInfo(u, &cfg, t.slog)

		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.InReleaseURLInfo{
				CfgName:    tc.conf,
				Origin:     fmt.Sprintf("%s://%s", u.Scheme, u.Host),
				Repository: tc.repo,
				Suite:      tc.suite,
			})
		} else {
			c.Assert(err, ErrorMatches, tc.errorMsg)
		}
	}
}

type packageURLInfoTest struct {
	url      string // The request URL
	conf     string // The repository configuration name
	repo     string // The repository name
	series   string // The distribution series
	errorMsg string // The error message
}

var packageURLInfoTests = []packageURLInfoTest{{
	url:      "http://archive.ubuntu.com/ubuntu/dists/focal/main/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	conf:     "default",
	repo:     "http://archive.ubuntu.com/ubuntu",
	series:   "focal",
	errorMsg: "",
}, {
	url:      "http://archive.not-ubuntu.com/ubuntu/dists/focal/main/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	conf:     "none",
	repo:     "http://archive.not-ubuntu.com/ubuntu",
	series:   "focal",
	errorMsg: "invalid repository: .*",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	conf:     "none",
	repo:     "http://archive.ubuntu.com/ubuntu",
	series:   "jammy",
	errorMsg: "invalid series: jammy",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/dists/focal/universe/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	conf:     "none",
	repo:     "http://archive.ubuntu.com/ubuntu",
	series:   "focal",
	errorMsg: "invalid component: universe",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/dists/focal/main/binary-amd64/Packages.gz",
	conf:     "default",
	repo:     "http://archive.ubuntu.com/ubuntu",
	series:   "focal",
	errorMsg: "",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/Packages.gz",
	conf:     "none",
	repo:     "http://archive.ubuntu.com/ubuntu",
	series:   "jammy",
	errorMsg: "invalid series: jammy",
}, {
	url:      "https://esm.ubuntu.com:443/fips-preview/ubuntu/dists/noble/main/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	conf:     "esm",
	repo:     "https://esm.ubuntu.com:443/fips-preview/ubuntu",
	series:   "noble",
	errorMsg: "",
}, {
	url:      "https://esm.ubuntu.com:443/other-repo/ubuntu/dists/noble/main/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	conf:     "none",
	repo:     "https://esm.ubuntu.com:443/fips-preview/ubuntu",
	series:   "noble",
	errorMsg: "invalid repository: https://esm.ubuntu.com:443/other-repo/ubuntu",
}}

func (t *configSuite) TestPackagesURLInfo(c *C) {
	for _, tc := range packageURLInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewPackagesURLInfo(u, &cfg, t.slog)

		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			if strings.Contains(tc.url, "/by-hash/") {
				c.Assert(info, DeepEquals, &config.PackagesURLInfo{
					CfgName:      tc.conf,
					Origin:       fmt.Sprintf("%s://%s", u.Scheme, u.Host),
					Repository:   tc.repo,
					Suite:        tc.series,
					Component:    "main",
					Architecture: "amd64",
					Digest:       "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
				})
			} else {
				c.Assert(info, DeepEquals, &config.PackagesURLInfo{
					CfgName:      tc.conf,
					Origin:       fmt.Sprintf("%s://%s", u.Scheme, u.Host),
					Repository:   tc.repo,
					Suite:        tc.series,
					Component:    "main",
					Architecture: "amd64",
					Digest:       "",
				})
			}
		} else {
			c.Assert(err, ErrorMatches, tc.errorMsg)
		}
	}
}

type translationURLInfoTest struct {
	url      string // The request URL
	conf     string // The repository configuration entry
	repo     string // The repository name (URL scheme and origin)
	series   string // The distribution series
	errorMsg string // The error message, if any
}

var translationURLInfoTests = []translationURLInfoTest{{
	url:      "http://archive.ubuntu.com/ubuntu/dists/focal/main/i18n/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	conf:     "default",
	repo:     "http://archive.ubuntu.com/ubuntu",
	series:   "focal",
	errorMsg: "",
}, {
	url:      "http://archive.not-ubuntu.com/ubuntu/dists/focal/main/i18n/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	conf:     "none",
	repo:     "http://archive.not-ubuntu.com/ubuntu",
	series:   "focal",
	errorMsg: "invalid repository: .*",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/dists/focal/main/binary-amd64/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	conf:     "none",
	repo:     "http://archive.ubuntu.com/ubuntu",
	series:   "focal",
	errorMsg: "invalid translation URL path: .*",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/dists/jammy/main/i18n/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	conf:     "none",
	repo:     "http://archive.ubuntu.com/ubuntu",
	series:   "jammy",
	errorMsg: "invalid series: jammy",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/dists/focal/universe/i18n/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	conf:     "none",
	repo:     "http://archive.ubuntu.com/ubuntu",
	series:   "focal",
	errorMsg: "invalid component: universe",
}, {
	url:      "https://esm.ubuntu.com:443/fips-preview/ubuntu/dists/noble/main/i18n/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	conf:     "esm",
	repo:     "https://esm.ubuntu.com:443/fips-preview/ubuntu",
	series:   "noble",
	errorMsg: "",
}, {
	url:      "https://esm.ubuntu.com:443/other-repo/ubuntu/dists/noble/main/i18n/by-hash/SHA256/5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	conf:     "none",
	repo:     "https://esm.ubuntu.com/other-repo/ubuntu",
	series:   "noble",
	errorMsg: "invalid repository: https://esm.ubuntu.com:443/other-repo/ubuntu",
}}

func (t *configSuite) TestTranslationURLInfo(c *C) {
	for _, tc := range translationURLInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewTranslationURLInfo(u, &cfg, t.slog)

		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.TranslationURLInfo{
				CfgName:    tc.conf,
				Origin:     fmt.Sprintf("%s://%s", u.Scheme, u.Host),
				Repository: tc.repo,
				Suite:      tc.series,
				Component:  "main",
				Digest:     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
			})
		} else {
			c.Assert(err, ErrorMatches, tc.errorMsg)
		}
	}
}

func (t *configSuite) TestTranslationURLInfoByName(c *C) {
	u, err := url.Parse("http://archive.ubuntu.com/ubuntu/dists/focal/main/i18n/Translation-en")
	c.Assert(err, IsNil)

	cfg := getTestAptConfig()
	info, err := config.NewTranslationURLInfo(u, &cfg, t.slog)

	c.Assert(err, IsNil)
	c.Assert(info, DeepEquals, &config.TranslationURLInfo{
		CfgName:    "default",
		Origin:     "http://archive.ubuntu.com",
		Repository: "http://archive.ubuntu.com/ubuntu",
		Suite:      "focal",
		Component:  "main",
		Digest:     "",
	})
}

type commandsURLInfoTest struct {
	url      string // The request URL
	conf     string // The repository configuration entry
	repo     string // The repository name (URL scheme and origin)
	series   string // The distribution series
	errorMsg string // The error message, if any
}

var commandsURLInfoTests = []commandsURLInfoTest{{
	url:      "http://archive.ubuntu.com/ubuntu/dists/focal/main/cnf/by-hash/SHA256/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
	conf:     "default",
	repo:     "http://archive.ubuntu.com/ubuntu",
	series:   "focal",
	errorMsg: "",
}, {
	url:      "http://archive.not-ubuntu.com/ubuntu/dists/focal/main/cnf/by-hash/SHA256/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
	conf:     "none",
	repo:     "http://archive.not-ubuntu.com/ubuntu",
	series:   "focal",
	errorMsg: "invalid repository: .*",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/dists/focal/main/binary-amd64/by-hash/SHA256/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
	conf:     "none",
	repo:     "http://archive.ubuntu.com/ubuntu",
	series:   "focal",
	errorMsg: "invalid commands URL path: .*",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/dists/jammy/main/cnf/by-hash/SHA256/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
	conf:     "none",
	repo:     "http://archive.ubuntu.com/ubuntu",
	series:   "jammy",
	errorMsg: "invalid series: jammy",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/dists/focal/universe/cnf/by-hash/SHA256/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
	conf:     "none",
	repo:     "http://archive.ubuntu.com/ubuntu",
	series:   "focal",
	errorMsg: "invalid component: universe",
}, {
	url:      "https://esm.ubuntu.com:443/fips-preview/ubuntu/dists/noble/main/cnf/by-hash/SHA256/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
	conf:     "esm",
	repo:     "https://esm.ubuntu.com:443/fips-preview/ubuntu",
	series:   "noble",
	errorMsg: "",
}, {
	url:      "https://esm.ubuntu.com:443/other-repo/ubuntu/dists/noble/main/cnf/by-hash/SHA256/6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
	conf:     "none",
	repo:     "https://esm.ubuntu.com/other-repo/ubuntu",
	series:   "noble",
	errorMsg: "invalid repository: https://esm.ubuntu.com:443/other-repo/ubuntu",
}}

func (t *configSuite) TestCommandsURLInfo(c *C) {
	for _, tc := range commandsURLInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewCommandURLInfo(u, &cfg, t.slog)

		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.CommandsURLInfo{
				CfgName:    tc.conf,
				Origin:     fmt.Sprintf("%s://%s", u.Scheme, u.Host),
				Repository: tc.repo,
				Suite:      tc.series,
				Component:  "main",
				Digest:     "6a94aa4e84721d193ff9e233a18293cc79a7659f903fcf2d7ba79fadc0877dbf",
			})
		} else {
			c.Assert(err, ErrorMatches, tc.errorMsg)
		}
	}
}

func (t *configSuite) TestCommandsURLInfoByName(c *C) {
	u, err := url.Parse("http://archive.ubuntu.com/ubuntu/dists/focal/main/cnf/Commands-amd64.xz")
	c.Assert(err, IsNil)

	cfg := getTestAptConfig()
	info, err := config.NewCommandURLInfo(u, &cfg, t.slog)

	c.Assert(err, IsNil)
	c.Assert(info, DeepEquals, &config.CommandsURLInfo{
		CfgName:    "default",
		Origin:     "http://archive.ubuntu.com",
		Repository: "http://archive.ubuntu.com/ubuntu",
		Suite:      "focal",
		Component:  "main",
		Digest:     "",
	})
}

type debPackageURLInfoTest struct {
	url      string // The request URL
	conf     string // The repository configuration entry
	repo     string // The repository URL scheme and origin
	errorMsg string // The error message if any
}

var debPackageURLInfoTests = []debPackageURLInfoTest{{
	url:      "http://archive.ubuntu.com/ubuntu/pool/main/c/curl/libcurl3-gnutls_7.81.0-1ubuntu1.16_amd64.deb",
	conf:     "default",
	repo:     "http://archive.ubuntu.com/ubuntu",
	errorMsg: "",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/pool/main/c/c/c/curl/libcurl3-gnutls_7.81.0-1ubuntu1.16_amd64.deb",
	conf:     "default",
	repo:     "http://archive.ubuntu.com/ubuntu",
	errorMsg: "",
}, {
	url:      "http://archive.not-ubuntu.com/ubuntu/pool/main/c/curl/libcurl3-gnutls_7.81.0-1ubuntu1.16_amd64.deb",
	conf:     "none",
	repo:     "http://archive.not-ubuntu.com/ubuntu",
	errorMsg: "invalid repository: http://archive.not-ubuntu.com/ubuntu",
}, {
	url:      "http://archive.ubuntu.com/ubuntu/dists/focal/main/c/curl/libcurl3-gnutls_7.81.0-1ubuntu1.16_amd64.deb",
	conf:     "none",
	repo:     "http://archive.ubuntu.com/ubuntu",
	errorMsg: "invalid repository URL: http://.*",
}, {
	url:      "https://esm.ubuntu.com:443/fips-preview/ubuntu/pool/main/c/curl/libcurl3-gnutls_7.81.0-1ubuntu1.16_amd64.deb",
	conf:     "esm",
	repo:     "https://esm.ubuntu.com:443/fips-preview/ubuntu",
	errorMsg: "",
}, {
	url:      "https://esm.ubuntu.com:443/other-repo/ubuntu/pool/main/c/curl/libcurl3-gnutls_7.81.0-1ubuntu1.16_amd64.deb",
	conf:     "none",
	repo:     "https://esm.ubuntu.com:443/other-repo/ubuntu",
	errorMsg: "invalid repository: https://esm.ubuntu.com:443/other-repo/ubuntu",
}}

func (t *configSuite) TestDebPackageURLInfo(c *C) {
	for _, tc := range debPackageURLInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewDebPackageURLInfo(u, &cfg, t.slog)

		if tc.errorMsg == "" {
			c.Assert(err, IsNil, Commentf("%+v", tc))
			c.Assert(info, DeepEquals, &config.DebPackageURLInfo{
				CfgName:      tc.conf,
				Origin:       fmt.Sprintf("%s://%s", u.Scheme, u.Host),
				Repository:   tc.repo,
				Component:    "main",
				Name:         "libcurl3-gnutls",
				Version:      "7.81.0-1ubuntu1.16",
				Architecture: "amd64",
			})
		} else {
			c.Assert(err, ErrorMatches, tc.errorMsg)
		}
	}
}
