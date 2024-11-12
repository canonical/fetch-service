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
	"os"
	"path/filepath"
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/cmd/fetchctl"
)

func (t *fetchctlSuite) TestUpdateConfig(c *C) {
	for _, tc := range []struct {
		optype   string
		dryRun   bool
		filename string
		result   string
		message  string
	}{
		{"acl", false, "file.txt", "ok", ""},
		{"invalid", false, "file.txt", "error", "unsupported type"},
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
			c.Check(string(data[:n]), Equals, fmt.Sprintf(`{"operation":"update-config","type":%q,"payload":"content"}`, tc.optype))

			_, err = f.Write([]byte(fmt.Sprintf(`{"result":%q,"message":%q}`, tc.result, tc.message)))
			c.Assert(err, IsNil)
			f.Close()
		}()

		time.Sleep(500 * time.Millisecond)
		filename := filepath.Join(tmpdir, tc.filename)
		err := os.WriteFile(filename, []byte("content"), 0644)
		c.Assert(err, IsNil)

		cmd := fetchctl.UpdateConfigCmd{
			Type:         tc.optype,
			ValidateOnly: false,
			Args: struct {
				Filename string `positional-arg-name:"filename"`
			}{
				filename,
			},
		}

		res := cmd.Execute([]string{"fetchctl", "update-config", "--type=acl", filename})

		if tc.result == "ok" {
			c.Assert(res, IsNil)
		} else {
			c.Assert(res, ErrorMatches, tc.message)
		}
	}
}
