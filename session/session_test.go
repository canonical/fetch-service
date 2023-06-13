// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023 Canonical Ltd.
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

package session_test

import (
	"testing"
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/session"
)

func Test(t *testing.T) { TestingT(t) }

type sessionSuite struct{}

var _ = Suite(&sessionSuite{})

func (t *sessionSuite) TestNewSession(c *C) {
	id_restorer := session.MockMakeSessionId(func() string {
		return "6ba7b8109dad11d180b400c04fd430c8"
	})
	defer id_restorer()

	rs_restorer := session.MockRandomString(func(int) string {
		return "1ItfzwGBeJ8wsJdP0Nlx"
	})
	defer rs_restorer()

	before := time.Now()
	s := session.New()
	after := time.Now()

	defer s.Discard()

	c.Assert(s.Id, Equals, "6ba7b8109dad11d180b400c04fd430c8")
	c.Assert(s.Pw, Equals, "1ItfzwGBeJ8wsJdP0Nlx")
	c.Assert(s.Start.After(before) || s.Start.Equal(before), Equals, true)
	c.Assert(s.Start.Before(after) || s.Start.Equal(after), Equals, true)
	c.Assert(s.End.Equal(time.Time{}), Equals, true)
	c.Assert(s, Equals, session.Sessions[s.Id])
}

func (t *sessionSuite) TestRandomString(c *C) {
	for _, n := range []int{0, 1, 10, 20} {
		x := session.RandomString(n)
		y := session.RandomString(n)
		c.Assert(len(x), Equals, n)
		c.Assert(len(y), Equals, n)
		c.Assert(x == y, Equals, n == 0) // only empty strings are equal
	}
}

func (t *sessionSuite) TestDiscardSession(c *C) {
	s := session.New()
	defer s.Discard()

	c.Assert(s, Equals, session.Sessions[s.Id])

	s.Discard()
	_, ok := session.Sessions[s.Id]
	c.Assert(ok, Equals, false)
}

func (t *sessionSuite) TestCheckAuth(c *C) {
	s := session.New()
	defer s.Discard()

	c.Assert(session.Sessions.CheckAuth("foo", "bar"), Equals, false)
	c.Assert(session.Sessions.CheckAuth(s.Id, s.Pw), Equals, true)
}

func (t *sessionSuite) TestIsActive(c *C) {
	s := session.New()
	defer s.Discard()

	c.Assert(session.IsActive("foo"), Equals, false)
	c.Assert(session.IsActive(s.Id), Equals, true)
	s.Discard()
	c.Assert(session.IsActive(s.Id), Equals, false)
}
