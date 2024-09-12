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

package git_test

import (
	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/git"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
)

type sourcecraftGitSuite struct{}

var _ = Suite(&sourcecraftGitSuite{})

func (t *sourcecraftGitSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

func (s *sourcecraftGitSuite) TestSourcecraftGitInspectorInterface(c *C) {
	var iface Inspector
	ins := git.NewSourcecraftInspector()
	c.Assert(ins, Implements, &iface)

}
