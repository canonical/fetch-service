// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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

package main_test

import (
	"fmt"
	"os"
	"testing"

	. "gopkg.in/check.v1"

	main "github.com/canonical/fetch-service/cmd/fetch"
	"github.com/canonical/fetch-service/version"
)

func Test(t *testing.T) { TestingT(t) }

type mainSuite struct {
}

var _ = Suite(&mainSuite{})

func (t *mainSuite) TestVersion(c *C) {
	output := []string{}
	restorer := main.MockPrintf(func(format string, a ...any) {
		line := fmt.Sprintf(format, a...)
		output = append(output, line)
	})
	defer restorer()

	restorer = main.MockArgs([]string{"fetch", "--version"})
	defer restorer()

	c.Assert(main.Run(), Equals, 0)
	c.Check(output, HasLen, 1)
	c.Check(output[0], Equals, "fetch "+version.Version+"\n")
}

func (t *mainSuite) TestOptionsNotSnapDefault(c *C) {
	_ = os.Unsetenv("SNAP_NAME")
	_ = os.Unsetenv("SNAP")

	opt := main.GetServiceOptions(main.CmdlineOptions{})

	// Default values in non-snap case
	c.Assert(opt.Config, Equals, "/etc/fetch")
	c.Assert(opt.Spool, Equals, "/var/lib/fetch")
}

func (t *mainSuite) TestOptionsSnapDefault(c *C) {
	_ = os.Setenv("SNAP_NAME", "fetch-service")
	_ = os.Setenv("SNAP", "/snap/fetch-service/x1")
	_ = os.Setenv("SNAP_DATA", "/var/snap/fetch-service/x1")
	_ = os.Setenv("SNAP_COMMON", "/var/snap/fetch-service/common")

	opt := main.GetServiceOptions(main.CmdlineOptions{})

	// Default values in snap case
	c.Assert(opt.Config, Equals, "/var/snap/fetch-service/x1/conf")
	c.Assert(opt.Spool, Equals, "/var/snap/fetch-service/common/spool")
}

func (t *mainSuite) TestOptionsUserSet(c *C) {
	opt := main.GetServiceOptions(main.CmdlineOptions{
		Spool:  "/user/spool",
		Config: "/user/config",
	})

	c.Assert(opt.Config, Equals, "/user/config")
	c.Assert(opt.Spool, Equals, "/user/spool")
}
