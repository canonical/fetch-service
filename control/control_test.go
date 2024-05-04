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

package control_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/control"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
)

func Test(t *testing.T) { TestingT(t) }

type controlSuite struct {
	ch chan any
}

func (t *controlSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&controlSuite{})

func (t *controlSuite) TestCheckAuth(c *C) {
	for _, tc := range []struct {
		cred string
		auth string
		res  bool
	}{
		{"", "", false},
		{"testuser:testpw", "Basic ", false},                     // no encoded credentials
		{"testuser:testpw", "Basic dGVzdHVzZXI6dGVzdHB2", false}, // bad encoded credentials
		{"testuser:testpq", "Basic dGVzdHVzZXI6dGVzdHB3", false}, // bad passwd
		{"testuser:testpw", "Basic dGVzdHVzZXI6dGVzdHB3", true},
	} {
		ctl := control.NewServer(1234, t.ch, tc.cred)

		w := httptest.NewRecorder()
		r, _ := http.NewRequest("GET", "http://some.url", nil)

		r.Header.Add("Authorization", tc.auth)
		res := control.ServerCheckAuth(ctl, w, r)
		c.Assert(res, Equals, tc.res)
	}
}

func (t *controlSuite) TestEndpointNoAuthentication(c *C) {
	ctl := control.NewServer(1234, t.ch, "testuser:testpw")

	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "http://some.url", nil)
	control.ServerDeleteSessionToken(ctl, w, r)
	c.Assert(w.Code, Equals, 404)
}

func (t *controlSuite) TestEndpointAuthenticationError(c *C) {
	for _, tc := range []struct {
		callEndpoint func(*control.Server, http.ResponseWriter, *http.Request)
	}{
		{control.ServerGetServiceStatus},
		{control.ServerCreateSession},
		{control.ServerDeleteSession},
		{control.ServerGetSessionReport},
		{control.ServerDeleteResources},
	} {
		ctl := control.NewServer(1234, t.ch, "testuser:testpw")

		w := httptest.NewRecorder()
		r, _ := http.NewRequest("GET", "http://some.url", nil)
		tc.callEndpoint(ctl, w, r)
		c.Assert(w.Code, Equals, 401)
	}
}
