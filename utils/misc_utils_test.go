// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2026 Canonical Ltd.
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
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/utils"
)

type miscUtilsSuite struct{}

var _ = Suite(&miscUtilsSuite{})

type serverIPTest struct {
	addr     net.Addr // Server address in request
	expected string   // The expected result
}

var serverIPTests = []serverIPTest{{
	addr:     &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 8080},
	expected: "10.0.0.1",
}, {
	addr:     nil,
	expected: "unknown",
}}

func (t *miscUtilsSuite) TestServerIP(c *C) {
	for _, tc := range serverIPTests {
		req := httptest.NewRequest("GET", "/", nil)
		if tc.addr != nil {
			ctx := context.WithValue(req.Context(), http.LocalAddrContextKey, tc.addr)
			req = req.WithContext(ctx)
		}
		c.Check(utils.ServerIP(req), Equals, tc.expected)
	}
}

type clientIPTest struct {
	remoteAddr string // Remote address in request
	expected   string // The expected result
}

var clientIPTests = []clientIPTest{{
	remoteAddr: "192.168.1.1:1234",
	expected:   "192.168.1.1",
}, {
	remoteAddr: "[2001:db8::1]:8080",
	expected:   "2001:db8::1",
}, {
	remoteAddr: "invalid",
	expected:   "address invalid: missing port in address (invalid)",
}}

func (t *miscUtilsSuite) TestClientIP(c *C) {
	for _, tc := range clientIPTests {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = tc.remoteAddr
		c.Check(utils.ClientIP(req), Equals, tc.expected)
	}
}

type runtimeEnvTest struct {
	value    string // The environment variable value
	expected string // The expected result
}

var runtimeEnvTests = []runtimeEnvTest{{
	value:    "production",
	expected: "production",
}, {
	value:    "",
	expected: "unknown",
}}

func (t *miscUtilsSuite) TestRuntimeEnv(c *C) {
	for _, tc := range runtimeEnvTests {
		appEnv := os.Getenv("APP_ENV")
		os.Setenv("APP_ENV", tc.value)
		defer os.Setenv("APP_ENV", appEnv)
		c.Check(utils.RuntimeEnv(), Equals, tc.expected)
	}
}
