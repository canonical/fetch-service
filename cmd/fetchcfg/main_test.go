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
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/cmd/fetchcfg"
)

func Test(t *testing.T) { TestingT(t) }

type fetchcfgSuite struct {
}

var _ = Suite(&fetchcfgSuite{})

func (t *fetchcfgSuite) TestVersion(c *C) {
	output := []string{}
	restorer := fetchcfg.MockPrintf(func(format string, a ...any) {
		line := fmt.Sprintf(format, a...)
		output = append(output, line)
	})
	defer restorer()

	restorer = fetchcfg.MockArgs([]string{"fetchcfg", "--version"})
	defer restorer()

	c.Assert(fetchcfg.Run(), Equals, 0)
	c.Check(output, HasLen, 1)
	c.Check(output[0], Equals, "fetchcfg "+fetchcfg.Version+"\n")
}
