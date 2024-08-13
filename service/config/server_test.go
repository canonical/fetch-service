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

package config_test

import (
	"encoding/json"
	"io/fs"
	"net"
	"os"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/service/config"
)

type serverSuite struct{}

var _ = Suite(&serverSuite{})

func (t *serverSuite) TestConfigServer(c *C) {
	cs := config.NewServer()

	err := cs.Start()
	c.Assert(err, IsNil)

	fi, err := os.Stat(config.SocketPath())
	c.Assert(err, IsNil)
	c.Check(fi.Mode()&fs.ModeSocket, Equals, fs.ModeSocket)

	err = cs.Stop()
	c.Assert(err, IsNil)
}

func (t *serverSuite) TestConfigServerConnect(c *C) {
	for _, tc := range []struct {
		request string
		errMsg  string
	}{
		{`{"operation":"foo", "type":"bar", "validate-only":false, "payload":"baz"}`, ""},
		{`not a valid json`, "bla"},
	} {
		cs := config.NewServer()

		err := cs.Start()
		c.Assert(err, IsNil)

		conn, err := net.Dial("unix", config.SocketPath())
		c.Assert(err, IsNil)

		_, err = conn.Write([]byte(tc.request))
		c.Assert(err, IsNil)

		data := make([]byte, 1024)
		n, err := conn.Read(data)
		c.Assert(err, IsNil)

		var reply config.OperationReply
		err = json.Unmarshal(data[:n], &reply)
		c.Assert(err, IsNil)

		if tc.errMsg == "" {
			c.Check(reply.Result, Equals, "ok")
		} else {
			c.Check(reply.Result, Equals, "error")
			c.Check(reply.Message, Matches, "invalid character .*")
		}

		err = cs.Stop()
		c.Assert(err, IsNil)
	}
}

func (t *serverSuite) TestBuildReply(c *C) {
	x := config.BuildReply("foo", "bar")
	c.Assert(string(x), Equals, `{"result":"foo","message":"bar"}`)
}

func (t *serverSuite) TestSocketPath(c *C) {
	x := config.SocketPath()
	c.Assert(strings.HasPrefix(x, os.TempDir()), Equals, true)
}
