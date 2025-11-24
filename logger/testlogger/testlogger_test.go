// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2025 Canonical Ltd.
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

package testlogger_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
)

type testloggerSuite struct {
}

var _ = Suite(&testloggerSuite{})

func Test(t *testing.T) { TestingT(t) }

func (t *testloggerSuite) TestTestLogger(c *C) {
	testlogger.Init(logger.InfoLevel)
	logger.Debugf("hello %s", "world")
	c.Assert(testlogger.Contains("hello world"), Equals, false)

	testlogger.Init(logger.DebugLevel)
	logger.Debugf("goodbye %s", "world")
	c.Assert(testlogger.Contains("goodbye world"), Equals, true)
}
