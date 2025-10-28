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

package config_test

import (
	"net/url"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/inspectors/store/config"
	"github.com/canonical/fetch-service/logger"
)

type configSuite struct {
	slog logger.Logger
}

var _ = Suite(&configSuite{logger.NewSessionLogger("test")})

func Test(t *testing.T) { TestingT(t) }

func getTestStoreConfig() config.StoreInspectorConfig {
	return config.StoreInspectorConfig{
		Urls: []glob.Glob{
			glob.MustCompile("https://api.snapcraft.io:443/v2/bins/info/**"),
			glob.MustCompile("https://api.snapcraft.io:443/v2/revisions/resolve"),
		},
	}
}

type storeInfoAPIUrlInfoTest struct {
	url    string // The request URL
	errMsg string // The error message, if any
}

var storeInfoAPIUrlInfoTests = []storeInfoAPIUrlInfoTest{{
	url:    "https://api.snapcraft.io:443/v2/bins/info/starcraft-test?fields=summary,description",
	errMsg: "",
}, {
	// Wrong version
	url:    "https://api.snapcraft.io:443/v3/bins/info/starcraft-test?fields=summary,description",
	errMsg: "invalid url .*",
}, {
	// Wrong host
	url:    "https://snapcraft.com:443/v2/bins/info/starcraft-test?fields=summary,description",
	errMsg: "invalid url .*",
}}

func (t *configSuite) TestStoreInfoAPIUrlInfo(c *C) {
	for _, tc := range storeInfoAPIUrlInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestStoreConfig()
		info, err := config.NewStoreInfoAPIUrlInfo(u, &cfg, t.slog)

		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.StoreInfoAPIUrlInfo{
				PackageType: "bins",
				PackageName: "starcraft-test",
			})
		} else {
			c.Assert(err, ErrorMatches, tc.errMsg)
		}
	}
}

type storeResolveAPIUrlResolveTest struct {
	url    string // The request URL
	errMsg string // The error message, if any
}

var storeResolveAPIUrlResolveTests = []storeResolveAPIUrlResolveTest{{
	url:    "https://api.snapcraft.io:443/v2/revisions/resolve",
	errMsg: "",
}, {
	// Wrong version
	url:    "https://api.snapcraft.io:443/v3/revisions/resolve",
	errMsg: "invalid url .*",
}, {
	// Wrong host
	url:    "https://snapcraft.com:443/v2/revisions/resolve",
	errMsg: "invalid url .*",
}}

func (t *configSuite) TestStoreResolveAPIUrlResolve(c *C) {
	for _, tc := range storeResolveAPIUrlResolveTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestStoreConfig()
		info, err := config.NewStoreResolveAPIUrlInfo(u, &cfg, t.slog)

		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.StoreResolveAPIUrlInfo{})
		} else {
			c.Assert(err, ErrorMatches, tc.errMsg)
		}
	}
}
