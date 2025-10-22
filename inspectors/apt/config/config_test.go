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
				Urls:       []glob.Glob{glob.MustCompile("http://*.ubuntu.com/ubuntu")},
				Suites:     []glob.Glob{glob.MustCompile("focal")},
				Components: []glob.Glob{glob.MustCompile("main")},
				PublicKey:  "",
			},
			"esm": {
				Urls:       []glob.Glob{glob.MustCompile("https://esm.ubuntu.com:443/fips*/ubuntu")},
				Suites:     []glob.Glob{glob.MustCompile("noble")},
				Components: []glob.Glob{glob.MustCompile("main")},
				PublicKey:  "",
			},
		},
	}
}

type inReleaseUrlInfoTest struct {
	url      string // The request URL
	conf     string // The repository configuration entry
	repo     string // The repository name (URL scheme and origin)
	suite    string // The distribution series
	errorMsg string // The error message, if any
}

var inReleaseUrlInfoTests = []inReleaseUrlInfoTest{{
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

func (t *configSuite) TestInReleaseUrlInfo(c *C) {
	for _, tc := range inReleaseUrlInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewInReleaseUrlInfo(u, &cfg, t.slog)

		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.InReleaseUrlInfo{
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

type packageUrlInfoTest struct {
	url      string // The request URL
	conf     string // The repository configuration name
	repo     string // The repository name
	series   string // The distribution series
	errorMsg string // The error message
}

var packageUrlInfoTests = []packageUrlInfoTest{{
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

func (t *configSuite) TestPackagesUrlInfo(c *C) {
	for _, tc := range packageUrlInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewPackagesUrlInfo(u, &cfg, t.slog)

		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			if strings.Contains(tc.url, "/by-hash/") {
				c.Assert(info, DeepEquals, &config.PackagesUrlInfo{
					CfgName:      tc.conf,
					Origin:       fmt.Sprintf("%s://%s", u.Scheme, u.Host),
					Repository:   tc.repo,
					Suite:        tc.series,
					Component:    "main",
					Architecture: "amd64",
					Digest:       "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
				})
			} else {
				c.Assert(info, DeepEquals, &config.PackagesUrlInfo{
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

type translationUrlInfoTest struct {
	url      string // The request URL
	conf     string // The repository configuration entry
	repo     string // The repository name (URL scheme and origin)
	series   string // The distribution series
	errorMsg string // The error message, if any
}

var translationUrlInfoTests = []translationUrlInfoTest{{
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

func (t *configSuite) TestTranslationUrlInfo(c *C) {
	for _, tc := range translationUrlInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewTranslationUrlInfo(u, &cfg, t.slog)

		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.TranslationUrlInfo{
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

type commandsUrlInfoTest struct {
	url      string // The request URL
	conf     string // The repository configuration entry
	repo     string // The repository name (URL scheme and origin)
	series   string // The distribution series
	errorMsg string // The error message, if any
}

var commandsUrlInfoTests = []commandsUrlInfoTest{{
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

func (t *configSuite) TestCommandsUrlInfo(c *C) {
	for _, tc := range commandsUrlInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewCommandsUrlInfo(u, &cfg, t.slog)

		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.CommandsUrlInfo{
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

func (t *configSuite) TestCommandsUrlInfoByName(c *C) {
	u, err := url.Parse("http://archive.ubuntu.com/ubuntu/dists/focal/main/cnf/Commands-amd64.xz")
	c.Assert(err, IsNil)

	cfg := getTestAptConfig()
	info, err := config.NewCommandsUrlInfo(u, &cfg, t.slog)

	c.Assert(err, IsNil)
	c.Assert(info, DeepEquals, &config.CommandsUrlInfo{
		CfgName:    "default",
		Origin:     fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		Repository: "http://archive.ubuntu.com/ubuntu",
		Suite:      "focal",
		Component:  "main",
		Digest:     "",
	})
}

type debPackageUrlInfoTest struct {
	url      string // The request URL
	conf     string // The repository configuration entry
	repo     string // The repository URL scheme and origin
	errorMsg string // The error message if any
}

var debPackageUrlInfoTests = []debPackageUrlInfoTest{{
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

func (t *configSuite) TestDebPackageUrlInfo(c *C) {
	for _, tc := range debPackageUrlInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestAptConfig()
		info, err := config.NewDebPackageUrlInfo(u, &cfg, t.slog)

		if tc.errorMsg == "" {
			c.Assert(err, IsNil, Commentf("%+v", tc))
			c.Assert(info, DeepEquals, &config.DebPackageUrlInfo{
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
