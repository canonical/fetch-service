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

package version_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/canonical/fetch-service/version"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type versionSuite struct {
}

func (t *versionSuite) SetUpTest(c *C) {
}

var _ = Suite(&versionSuite{})

func (t *versionSuite) TestVersion(c *C) {
	out, err := exec.Command("git", "describe").Output()
	c.Assert(err, IsNil)
	c.Check(version.Version, Equals, strings.TrimSpace(string(out)))
}
