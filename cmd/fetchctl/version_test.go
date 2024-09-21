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
	"net"
	"path/filepath"
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/cmd/fetchctl"
)

func (t *fetchctlSuite) TestVersion(c *C) {
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
		c.Check(string(data[:n]), Equals, `{"operation":"version","payload":""}`)

		_, err = f.Write([]byte(`{"result":"ok","message":"1.2.3"}`))
		c.Assert(err, IsNil)
		f.Close()
	}()

	time.Sleep(500 * time.Millisecond)

	cmd := fetchctl.VersionCmd{}
	c.Assert(cmd.Execute([]string{"fetchctl", "version"}), IsNil)
}
