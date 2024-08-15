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

package main_test

import (
	"fmt"
	"testing"

	. "gopkg.in/check.v1"

	main "github.com/canonical/fetch-service/cmd/fetch"
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
	c.Check(output[0], Equals, "fetch "+main.Version+"\n")
}
