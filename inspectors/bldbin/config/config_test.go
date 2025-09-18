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
	"github.com/canonical/fetch-service/inspectors/bldbin/config"
	"github.com/canonical/fetch-service/logger"
)

type configSuite struct {
	slog logger.Logger
}

var _ = Suite(&configSuite{logger.NewSessionLogger("test")})

func Test(t *testing.T) { TestingT(t) }

func getTestBldBinConfig() config.BldBinInspectorConfig {
	return config.BldBinInspectorConfig{
		Urls: []glob.Glob{
			glob.MustCompile("https://api.snapcraft.io:443/api/v1/bins/download/**"),
		},
	}
}

type bldBinUrlInfoTest struct {
	url    string // The request URL
	errMsg string // The error message, if any
}

var bldBinUrlInfoTests = []bldBinUrlInfoTest{{
	url:    "https://api.snapcraft.io:443/api/v1/bins/download/package_1.0.bin",
	errMsg: "",
}, {
	url:    "https://snapcraft.com:443/api/v2/bins/download/package_1.0.bin",
	errMsg: "invalid url .*",
}, {
	url:    "https://snapcraft.com:443/api/v1/bins/info/package_1.0.bin",
	errMsg: "invalid url .*",
}, {
	url:    "https://snapcraft.com:443/api/v1/bins/download/package_1.0.bin",
	errMsg: "invalid url .*",
}}

func (t *configSuite) TestBldBinUrlInfo(c *C) {
	for _, tc := range bldBinUrlInfoTests {
		u, err := url.Parse(tc.url)
		c.Assert(err, IsNil)

		cfg := getTestBldBinConfig()
		info, err := config.NewBldBinUrlInfo(u, &cfg, t.slog)

		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(info, DeepEquals, &config.BldBinUrlInfo{})
		} else {
			c.Assert(err, ErrorMatches, tc.errMsg)
		}
	}
}
