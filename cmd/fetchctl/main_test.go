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
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/cmd/fetchctl"
)

func Test(t *testing.T) { TestingT(t) }

type fetchctlSuite struct {
}

var _ = Suite(&fetchctlSuite{})

func (t *fetchctlSuite) TestRun(c *C) {
	restorer := fetchctl.MockArgs([]string{"fetchctl", "invalid"})
	defer restorer()

	res := fetchctl.Run()
	c.Assert(res, Equals, 1)
}
