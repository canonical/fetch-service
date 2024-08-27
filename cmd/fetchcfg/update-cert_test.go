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

package fetchcfg_test

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/cmd/fetchcfg"
)

func (t *fetchcfgSuite) TestUpdateCert(c *C) {
	for _, tc := range []struct {
		filename string
		result   string
		message  string
	}{
		{"file.txt", "ok", ""},
		{"file.txt", "error", "something failed"},
	} {
		tmpdir := c.MkDir()
		spath := filepath.Join(tmpdir, "test.socket")
		restorer := fetchcfg.MockConfigSocketPath(func() string {
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
			c.Check(string(data[:n]), Equals, `{"operation":"update-cert","payload":"content"}`)

			_, err = f.Write([]byte(fmt.Sprintf(`{"result":%q,"message":%q}`, tc.result, tc.message)))
			c.Assert(err, IsNil)
		}()

		time.Sleep(500 * time.Millisecond)
		filename := filepath.Join(tmpdir, tc.filename)
		err := os.WriteFile(filename, []byte("content"), 0644)
		c.Assert(err, IsNil)

		cmd := fetchcfg.UpdateCertCmd{
			ValidateOnly: false,
			Args: struct {
				Filename string `positional-arg-name:"filename"`
			}{
				filename,
			},
		}

		res := cmd.Execute([]string{"fetchcfg", "update-cert", filename})

		if tc.result == "ok" {
			c.Assert(res, IsNil)
		} else {
			c.Assert(res, ErrorMatches, tc.message)
		}
	}
}
