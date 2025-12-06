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
 */

package git_test

import (
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/git"
	"github.com/canonical/fetch-service/logger"
)

type utilsSuite struct {
	slog logger.Logger
}

var _ = Suite(&utilsSuite{logger.NewSessionLogger("test")})

type unpackObjectsTest struct {
	testfile string // The file containing test data
	errorMsg string // The expected error message, if any
}

var unpackObjectsTests = []unpackObjectsTest{{
	testfile: "testdata/sourcepkg.raw",
	errorMsg: "",
}, {
	testfile: "testdata/bad-data.raw",
	errorMsg: ".* invalid syntax",
}, {
	testfile: "testdata/only-sideband.raw",
	errorMsg: "",
}}

func (s *utilsSuite) TestUnpackObjects(c *C) {
	for _, tc := range unpackObjectsTests {
		dir := c.MkDir()
		f, err := os.Open(tc.testfile)
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		err = git.UnpackObjects(f, dir, s.slog)
		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			_, err = os.Stat(filepath.Join(dir, ".git", "objects"))
			c.Assert(err, IsNil)
		} else {
			c.Assert(err, ErrorMatches, tc.errorMsg)
		}

	}
}

type checkoutTest struct {
	wants    string // The wanted ref
	errorMsg string // The expected error message, if any
}

var checkoutTests = []checkoutTest{{
	wants:    "10fce2c8e3a341998ffd2aa4e27b02699d1bb5ad",
	errorMsg: "",
}, {
	wants:    "not-a-valid-ref",
	errorMsg: "exit status 1",
}}

func (s *utilsSuite) TestCheckout(c *C) {
	for _, tc := range checkoutTests {
		dir := c.MkDir()
		f, err := os.Open("testdata/sourcepkg.raw")
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		err = git.UnpackObjects(f, dir, s.slog)
		c.Assert(err, IsNil)
		err = git.Checkout(dir, tc.wants, s.slog)
		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			_, err = os.Stat(filepath.Join(dir, "sourcecraft.yaml"))
			c.Assert(err, IsNil)
		} else {
			c.Assert(err, ErrorMatches, tc.errorMsg)
		}
	}
}
