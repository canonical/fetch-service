// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023 Canonical Ltd.
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

package logger_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
)

func Test(t *testing.T) { TestingT(t) }

type logSuite struct {
}

func (t *logSuite) SetUpTest(c *C) {
	testlogger.Init(logger.DebugLevel)
}

var _ = Suite(&logSuite{})

func (s *logSuite) TestInfo(c *C) {
	logger.Info("hello world")
	c.Check(testlogger.Contains("INFO : hello world\n"), Equals, true)
}

func (s *logSuite) TestInfof(c *C) {
	logger.Infof("hello %s", "world")
	c.Check(testlogger.Contains("INFO : hello world\n"), Equals, true)
}

func (s *logSuite) TestWarning(c *C) {
	logger.Warning("hello world")
	c.Check(testlogger.Contains("WARN : hello world\n"), Equals, true)
}

func (s *logSuite) TestWarningf(c *C) {
	logger.Warningf("hello %s", "world")
	c.Check(testlogger.Contains("WARN : hello world\n"), Equals, true)
}

func (s *logSuite) TestError(c *C) {
	logger.Error("hello world")
	c.Check(testlogger.Contains("ERROR: hello world\n"), Equals, true)
}

func (s *logSuite) TestErrorf(c *C) {
	logger.Errorf("hello %s", "world")
	c.Check(testlogger.Contains("ERROR: hello world\n"), Equals, true)
}

func (s *logSuite) TestDebug(c *C) {
	logger.Debug("hello world")
	c.Check(testlogger.Contains("DEBUG: hello world\n"), Equals, true)
}

func (s *logSuite) TestDebugf(c *C) {
	logger.Debugf("hello %s", "world")
	c.Check(testlogger.Contains("DEBUG: hello world\n"), Equals, true)
}

func (s *logSuite) TestInfoLevel(c *C) {
	tc := []struct {
		l   logger.Level
		res bool
	}{
		{logger.DebugLevel, true},
		{logger.InfoLevel, true},
		{logger.WarningLevel, false},
		{logger.ErrorLevel, false},
	}

	for _, t := range tc {
		testlogger.Init(t.l)
		logger.Info("string")
		logger.Infof("formatted")

		c.Check(testlogger.Contains("INFO : string\n"), Equals, t.res)
		c.Check(testlogger.Contains("INFO : formatted\n"), Equals, t.res)
	}
}

func (s *logSuite) TestWarningLevel(c *C) {
	tc := []struct {
		l   logger.Level
		res bool
	}{
		{logger.DebugLevel, true},
		{logger.InfoLevel, true},
		{logger.WarningLevel, true},
		{logger.ErrorLevel, false},
	}

	for _, t := range tc {
		testlogger.Init(t.l)
		logger.Warning("string")
		logger.Warningf("formatted")

		c.Check(testlogger.Contains("WARN : string\n"), Equals, t.res)
		c.Check(testlogger.Contains("WARN : formatted\n"), Equals, t.res)
	}
}

func (s *logSuite) TestErrorLevel(c *C) {
	tc := []struct {
		l   logger.Level
		res bool
	}{
		{logger.DebugLevel, true},
		{logger.InfoLevel, true},
		{logger.ErrorLevel, true},
		{logger.ErrorLevel, true},
	}

	for _, t := range tc {
		testlogger.Init(t.l)
		logger.Error("string")
		logger.Errorf("formatted")

		c.Check(testlogger.Contains("ERROR: string\n"), Equals, t.res)
		c.Check(testlogger.Contains("ERROR: formatted\n"), Equals, t.res)
	}
}

func (s *logSuite) TestDebugLevel(c *C) {
	tc := []struct {
		l   logger.Level
		res bool
	}{
		{logger.DebugLevel, true},
		{logger.InfoLevel, false},
		{logger.ErrorLevel, false},
		{logger.ErrorLevel, false},
	}

	for _, t := range tc {
		testlogger.Init(t.l)
		logger.Debug("string")
		logger.Debugf("formatted")

		c.Check(testlogger.Contains("DEBUG: string\n"), Equals, t.res)
		c.Check(testlogger.Contains("DEBUG: formatted\n"), Equals, t.res)
	}
}

func (s *logSuite) TestLogToFile(c *C) {
	logDir := c.MkDir()
	logPath := filepath.Join(logDir, "testfile.log")

	err := logger.Init(logger.DebugLevel, logPath)
	c.Assert(err, IsNil)

	logger.Debug("Debug message")
	logger.Info("Info message")
	logger.Warning("Warning message")
	logger.Close()
	logger.Info("No message")

	buf, err := os.ReadFile(logPath)
	c.Assert(err, IsNil)

	contents := string(buf)
	lines := strings.Split(contents, "\n")
	c.Assert(strings.Contains(lines[0], "DEBUG: Debug message"), Equals, true)
	c.Assert(strings.Contains(lines[1], "INFO : Info message"), Equals, true)
	c.Assert(strings.Contains(lines[2], "WARN : Warning message"), Equals, true)
	c.Assert(lines[3], Equals, "")
}

func (s *logSuite) TestLogToFileError(c *C) {
	err := logger.Init(logger.DebugLevel, "/invalid/path/log.txt")
	c.Assert(err, ErrorMatches, "set log file:.* /invalid/path/log.txt: no such file or directory.*")
}
