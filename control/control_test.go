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
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/control"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/service/messages"
)

func Test(t *testing.T) { TestingT(t) }

type controlSuite struct {
}

func (t *controlSuite) SetUpTest(c *C) {
}

var _ = Suite(&controlSuite{})

func (t *controlSuite) TestStartStop(c *C) {
	ch := make(chan any, 1)
	ctl := control.NewServer(18111, ch, "user:password")
	ctl.Start()

	c.Assert(ctl.Alive(), Equals, true)

	err := ctl.Stop()
	c.Assert(err, IsNil)

	c.Assert(ctl.Alive(), Equals, false)
}

func (t *controlSuite) TestServerError(c *C) {
	ch := make(chan any, 1)
	ctl := control.NewServer(18111, ch, "user:password")
	ctl.Start()

	c.Assert(ctl.Alive(), Equals, true)

	err := errors.New("an error")
	ctl.ForceError(err)
	c.Assert(ctl.Err(), Equals, err)

	c.Assert(ctl.Alive(), Equals, false)
}

func (t *controlSuite) TestServerListening(c *C) {
	ch := make(chan any, 1)
	ctl := control.NewServer(18111, ch, "user:password")
	ctl.Start()

	c.Assert(ctl.Alive(), Equals, true)

	ctl2 := control.NewServer(18111, ch, "user:password")
	ctl2.Start()

	time.Sleep(500 * time.Millisecond)

	c.Assert(ctl2.Err(), ErrorMatches, ".* bind: address already in use")
	c.Assert(ctl2.Alive(), Equals, false)
}

func (t *controlSuite) TestCreateSession(c *C) {
	for _, tc := range []struct {
		params  string // request body parameters
		errmsg  string // session creation error
		resCode int    // expected result code
	}{
		{`{"policy": "permissive"}`, "", 200},
		{`{"policy": "permissive"}`, "oops", 400}, // session creation error
		{"not json", "", 400},                     // bad parameters
	} {
		ch := make(chan any, 1)
		server := control.NewServer(3333, ch, "foo:bar")
		body := bytes.NewReader([]byte(tc.params))
		req, err := http.NewRequest("POST", "http://localhost/session", body)
		c.Assert(err, IsNil)
		req.Header["Authorization"] = []string{"Basic Zm9vOmJhcg=="}

		go func() {
			msg := <-ch
			cred := messages.SessionCredentials{Id: "A", Token: "B"}
			if tc.errmsg != "" {
				cred.Err = errors.New(tc.errmsg)
			}
			msg.(messages.CreateSession).Rch <- cred
		}()

		w := httptest.NewRecorder()
		control.ServerCreateSession(server, w, req)
		c.Check(w.Code, Equals, tc.resCode)

		if tc.resCode == 200 {
			var cred messages.SessionCredentials
			err = json.Unmarshal(w.Body.Bytes(), &cred)
			c.Assert(err, IsNil)
			c.Check(cred, DeepEquals, messages.SessionCredentials{Id: "A", Token: "B"})
		}
	}
}

func (t *controlSuite) TestDeleteSessionToken(c *C) {
	for _, tc := range []struct {
		idvar   string // id encoded in the URL
		body    string // request body
		err     error  // end session error
		resCode int    // expected result code
	}{
		{"012345", `{"token": "shalamacookie"}`, nil, 200},
		{"", `{"token": "shalamacookie"}`, nil, 404},                                   // session ID not in parameters
		{"012345", `{"token": "shalamacookie"}`, messages.ErrSessionNotFound, 404},     // session does not exist
		{"012345", `{"token": "shalamacookie"}`, messages.ErrInvalidSessionToken, 400}, // token does not exist
		{"012345", "not json", nil, 400},                                               // bad parameters
		{"012345", `{"token": "shalamacookie"}`, errors.New("other error"), 500},       // something else
	} {
		ch := make(chan any, 1)
		server := control.NewServer(3333, ch, "foo:bar")
		body := bytes.NewReader([]byte(tc.body))
		req, err := http.NewRequest("DELETE", "http://localhost/session/some-session-id/token", body)
		c.Assert(err, IsNil)
		if tc.idvar != "" {
			req = mux.SetURLVars(req, map[string]string{"id": "012345"})
		}
		req.Header["Authorization"] = []string{"Basic Zm9vOmJhcg=="}

		go func() {
			msg := <-ch
			msg.(messages.RevokeToken).Rch <- messages.RevokeTokenResult{
				SessionId: tc.idvar,
				Err:       tc.err,
			}
		}()

		w := httptest.NewRecorder()
		control.ServerDeleteSessionToken(server, w, req)
		c.Check(w.Code, Equals, tc.resCode)

		if tc.resCode == 200 {
			var res messages.RevokeTokenResult
			err = json.Unmarshal(w.Body.Bytes(), &res)
			c.Assert(err, IsNil)
			c.Check(res.Err, IsNil)
			c.Check(res.SessionId, Equals, tc.idvar)
		}
	}
}

func (t *controlSuite) TestGetSessionReport(c *C) {
	for _, tc := range []struct {
		idvar   string // id encoded in the URL
		err     error  // end session error
		resCode int    // expected result code
	}{
		{"012345", nil, 200},
		{"", nil, 404}, // session ID not in parameters
		{"012345", messages.ErrSessionNotFound, 404}, // session does not exist
		{"012345", messages.ErrSessionActive, 400},   // token does not exist
		{"012345", errors.New("other error"), 500},   // something else
	} {
		ch := make(chan any, 1)
		server := control.NewServer(3333, ch, "foo:bar")
		req, err := http.NewRequest("GET", "http://localhost/session/some-session-id", nil)
		c.Assert(err, IsNil)
		if tc.idvar != "" {
			req = mux.SetURLVars(req, map[string]string{"id": "012345"})
		}
		req.Header["Authorization"] = []string{"Basic Zm9vOmJhcg=="}

		go func() {
			artefacts := []*metadata.Artefact{metadata.NewArtefact()}
			artefacts[0].Metadata.AuthorEmail = "Jürgen <juergen@example.com>"

			msg := <-ch
			msg.(messages.SessionReport).Rch <- messages.SessionReportResult{
				Err:       tc.err,
				Artefacts: artefacts,
			}
		}()

		w := httptest.NewRecorder()
		control.ServerGetSessionReport(server, w, req)
		c.Check(w.Code, Equals, tc.resCode)

		if tc.resCode == 200 {
			var res messages.SessionReportResult
			err = json.Unmarshal(w.Body.Bytes(), &res)
			c.Assert(err, IsNil)
			c.Check(res.Err, IsNil)
			c.Check(res.Artefacts[0].Metadata.AuthorEmail, Equals, "Jürgen <juergen@example.com>")
		}
	}
}

func (t *controlSuite) TestDeleteSession(c *C) {
	for _, tc := range []struct {
		idvar   string // id encoded in the URL
		err     error  // end session error
		resCode int    // expected result code
	}{
		{"012345", nil, 200},
		{"", nil, 404}, // session ID not in parameters
		{"012345", messages.ErrSessionNotFound, 404}, // session does not exist
		{"012345", errors.New("other error"), 500},   // something else
	} {
		ch := make(chan any, 1)
		server := control.NewServer(3333, ch, "foo:bar")
		req, err := http.NewRequest("DELETE", "http://localhost/session/some-session-id", nil)
		c.Assert(err, IsNil)
		if tc.idvar != "" {
			req = mux.SetURLVars(req, map[string]string{"id": "012345"})
		}
		req.Header["Authorization"] = []string{"Basic Zm9vOmJhcg=="}

		go func() {
			msg := <-ch
			msg.(messages.EndSession).Rch <- tc.err
		}()

		w := httptest.NewRecorder()
		control.ServerDeleteSession(server, w, req)
		c.Check(w.Code, Equals, tc.resCode)
	}
}

func (t *controlSuite) TestDeleteResources(c *C) {
	for _, tc := range []struct {
		idvar   string // id encoded in the URL
		err     error  // end session error
		resCode int    // expected result code
	}{
		{"012345", nil, 200},
		{"", nil, 404}, // session ID not in parameters
		{"012345", messages.ErrSessionNotFound, 404}, // session does not exist
		{"012345", messages.ErrSessionActive, 400},   // session is still active
		{"012345", errors.New("other error"), 500},   // something else
	} {
		ch := make(chan any, 1)
		server := control.NewServer(3333, ch, "foo:bar")
		req, err := http.NewRequest("DELETE", "http://localhost/resources/some-session-id", nil)
		c.Assert(err, IsNil)
		if tc.idvar != "" {
			req = mux.SetURLVars(req, map[string]string{"id": "012345"})
		}
		req.Header["Authorization"] = []string{"Basic Zm9vOmJhcg=="}

		go func() {
			msg := <-ch
			msg.(messages.DeleteResources).Rch <- tc.err
		}()

		w := httptest.NewRecorder()
		control.ServerDeleteResources(server, w, req)
		c.Check(w.Code, Equals, tc.resCode)
	}
}

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
		ch := make(chan any, 1)
		ctl := control.NewServer(1234, ch, tc.cred)

		w := httptest.NewRecorder()
		r, _ := http.NewRequest("GET", "http://some.url", nil)

		r.Header.Add("Authorization", tc.auth)
		res := control.ServerCheckAuth(ctl, w, r)
		c.Assert(res, Equals, tc.res)
	}
}

func (t *controlSuite) TestEndpointNoAuthentication(c *C) {

	for _, tc := range []struct {
		callEndpoint func(*control.Server, http.ResponseWriter, *http.Request)
	}{
		{control.ServerGetServiceStatus},
		{control.ServerDeleteSessionToken},
	} {
		ch := make(chan any, 1)
		ctl := control.NewServer(1234, ch, "testuser:testpw")

		go func() {
			msg := <-ch
			msg.(messages.GetServiceStatus).Rch <- messages.ServiceStatus{}
		}()

		w := httptest.NewRecorder()
		r, _ := http.NewRequest("GET", "http://some.url", nil)
		tc.callEndpoint(ctl, w, r)
		c.Assert(w.Code, Not(Equals), 401)
	}
}

func (t *controlSuite) TestEndpointAuthenticationError(c *C) {
	for _, tc := range []struct {
		callEndpoint func(*control.Server, http.ResponseWriter, *http.Request)
	}{
		{control.ServerCreateSession},
		{control.ServerDeleteSession},
		{control.ServerGetSessionReport},
		{control.ServerDeleteResources},
	} {
		ch := make(chan any, 1)
		ctl := control.NewServer(1234, ch, "testuser:testpw")

		w := httptest.NewRecorder()
		r, _ := http.NewRequest("GET", "http://some.url", nil)
		tc.callEndpoint(ctl, w, r)
		c.Assert(w.Code, Equals, 401)
	}
}
