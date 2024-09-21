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

package fetchctl_test

import (
	"fmt"
	"net"
	"path/filepath"
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/cmd/fetchctl"
)

func (t *fetchctlSuite) TestCreateSession(c *C) {
	for _, tc := range []struct {
		sid        string
		token      string
		timeout    int
		permissive bool
		payload    string
		result     string
		errmsg     string
	}{
		{"", "", 0, false, "::0:strict", "ok", ""},
		{"session-id", "", 0, false, "session-id::0:strict", "ok", ""},
		{"", "token", 0, false, ":token:0:strict", "ok", ""},
		{"", "", 31415, false, "::31415:strict", "ok", ""},
		{"", "", 0, true, "::0:permissive", "ok", ""},
		{"session-id", "token", 31415, true, "session-id:token:31415:permissive", "ok", ""},
		{"", "", 0, false, "::0:strict", "error", "something wrong happened"},
	} {
		tmpdir := c.MkDir()
		spath := filepath.Join(tmpdir, "test.socket")
		restorer := fetchctl.MockFetchctlSocketPath(func() string {
			return spath
		})
		defer restorer()

		go func() {
			ln, err := net.Listen("unix", spath)
			c.Assert(err, IsNil)

			f, err := ln.Accept()
			c.Assert(err, IsNil)

			data := make([]byte, 4096)
			n, err := f.Read(data)
			c.Assert(err, IsNil)
			c.Check(string(data[:n]), Equals, fmt.Sprintf(`{"operation":"create-session","payload":%q}`, tc.payload))

			_, err = f.Write([]byte(fmt.Sprintf(`{"result":%q,"message":%q}`, tc.result, tc.errmsg)))
			c.Assert(err, IsNil)
			f.Close()
		}()

		time.Sleep(500 * time.Millisecond)

		cmd := fetchctl.CreateSessionCmd{
			SessionId:  tc.sid,
			Token:      tc.token,
			Timeout:    tc.timeout,
			Permissive: tc.permissive,
		}

		err := cmd.Execute([]string{"fetchctl", "create-session"}) // only argv[0] is relevant

		if tc.result == "ok" {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err.Error(), Equals, tc.errmsg)
		}
	}
}
