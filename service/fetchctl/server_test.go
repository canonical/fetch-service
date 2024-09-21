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
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"os"
	"strings"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/service/fetchctl"
	"github.com/canonical/fetch-service/service/messages"
)

type serverSuite struct{}

var _ = Suite(&serverSuite{})

func (t *serverSuite) TestfetchctlServer(c *C) {
	ch := make(chan interface{})
	cs := fetchctl.NewServer(ch)

	err := cs.Start()
	c.Assert(err, IsNil)

	fi, err := os.Stat(fetchctl.SocketPath())
	c.Assert(err, IsNil)
	c.Check(fi.Mode()&fs.ModeSocket, Equals, fs.ModeSocket)

	err = cs.Stop()
	c.Assert(err, IsNil)
}

func (t *serverSuite) TestfetchctlServerError(c *C) {
	ch := make(chan interface{})
	cs := fetchctl.NewServer(ch)

	err := cs.Start()
	c.Assert(err, IsNil)

	err = errors.New("an error")
	cs.ForceError(err)
	c.Assert(cs.Err(), Equals, err)
}

func (t *serverSuite) TestfetchctlServerConnect(c *C) {
	for _, tc := range []struct {
		request string
		errMsg  string
	}{
		{`{"operation":"version", "payload":""}`, ""},
		{`not a valid json`, "invalid character .*"},
	} {
		ch := make(chan interface{})
		cs := fetchctl.NewServer(ch)

		go func() {
			v := <-ch
			op := v.(messages.FetchCtl)
			if op.Operation == "version" {
				op.Rch <- messages.FetchCtlResult{Status: "ok", Message: ""}
			} else {
				op.Rch <- messages.FetchCtlResult{Status: "error", Message: ""}
			}

		}()

		err := cs.Start()
		c.Assert(err, IsNil)

		conn, err := net.Dial("unix", fetchctl.SocketPath())
		c.Assert(err, IsNil)

		_, err = conn.Write([]byte(tc.request))
		c.Assert(err, IsNil)

		data := make([]byte, 1024)
		n, err := conn.Read(data)
		c.Assert(err, IsNil)

		var reply fetchctl.OperationReply
		err = json.Unmarshal(data[:n], &reply)
		c.Assert(err, IsNil)

		if tc.errMsg == "" {
			c.Check(reply.Result, Equals, "ok")
		} else {
			c.Check(reply.Result, Equals, "error")
			c.Check(reply.Message, Matches, tc.errMsg)
		}

		err = cs.Stop()
		c.Assert(err, IsNil)
	}
}

func (t *serverSuite) TestBuildReply(c *C) {
	x := fetchctl.BuildReply("foo", "bar")
	c.Assert(string(x), Equals, `{"result":"foo","message":"bar"}`)
}

func (t *serverSuite) TestSocketPath(c *C) {
	x := fetchctl.SocketPath()
	c.Assert(strings.HasPrefix(x, os.TempDir()), Equals, true)
}
