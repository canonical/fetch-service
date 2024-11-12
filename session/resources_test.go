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

package session_test

import (
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/session"
)

func (t *sessionSuite) TestSessionMetadataWritten(c *C) {
	for _, tc := range []struct {
		hasSessionMetadata bool
		expectedResult     bool
	}{
		{true, true},
		{false, false},
	} {
		spool := c.MkDir()

		err := os.MkdirAll(filepath.Join(spool, "my-session-id"), 0755)
		c.Assert(err, IsNil)

		if tc.hasSessionMetadata {
			err = os.WriteFile(filepath.Join(spool, "my-session-id", "session.json"), []byte("content"), 0644)
			c.Assert(err, IsNil)
		}

		res := session.SessionMetadataWritten(spool, "my-session-id")
		c.Assert(res, Equals, tc.expectedResult)
	}
}

func (t *sessionSuite) TestLoadSessionMetadata(c *C) {
	for _, tc := range []struct {
		hasSessionMetadata bool
		content            string
		errMsg             string
	}{
		{true, `{"session-id": "my-session-id"}`, ""},
		{false, "", "open .*: no such file or directory"},
		{true, "invalid-content", "invalid character 'i' .*"},
	} {
		spool := c.MkDir()

		err := os.MkdirAll(filepath.Join(spool, "my-session-id"), 0755)
		c.Assert(err, IsNil)

		if tc.hasSessionMetadata {
			err = os.WriteFile(filepath.Join(spool, "my-session-id", "session.json"), []byte(tc.content), 0644)
			c.Assert(err, IsNil)
		}

		sm, err := session.LoadSessionMetadata(spool, "my-session-id")
		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(sm.SessionId, Equals, "my-session-id")
		} else {
			c.Assert(err, ErrorMatches, tc.errMsg)
		}
	}
}

func (t *sessionSuite) TestRemoveResources(c *C) {
	for _, tc := range []struct {
		hasSessionDir bool
		errMsg        string
	}{
		{true, ""},
		{false, ""},
	} {

		spool := c.MkDir()
		tf := filepath.Join(spool, "my-session-id", "foo")

		var err error
		if tc.hasSessionDir {
			err = os.MkdirAll(filepath.Join(spool, "my-session-id"), 0755)
			c.Assert(err, IsNil)

			err = os.WriteFile(tf, []byte("content"), 0644)
			c.Assert(err, IsNil)

			_, err = os.Stat(tf)
			c.Assert(err, IsNil)
		} else {
			_, err = os.Stat(tf)
			c.Assert(err, ErrorMatches, "stat .*: no such file or directory")
		}

		err = session.RemoveResources(spool, "my-session-id")
		c.Assert(err, IsNil)

		_, err = os.Stat(tf)
		c.Assert(err, ErrorMatches, "stat .*: no such file or directory")
	}
}
