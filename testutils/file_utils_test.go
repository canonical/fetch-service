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

package testutils_test

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/testutils"
)

type fileUtilsSuite struct {
}

func (t *fileUtilsSuite) SetUpTest(c *C) {
}

func (t *fileUtilsSuite) TearDownTest(c *C) {
}

var _ = Suite(&fileUtilsSuite{})

func (t *fileUtilsSuite) TestCreateZip(c *C) {
	tmp := c.MkDir()
	zipFile := filepath.Join(tmp, "foo.zip")

	file := filepath.Join(tmp, "somedir", "bar.txt")
	err := os.MkdirAll(filepath.Dir(file), 0755)
	c.Assert(err, IsNil)

	err = os.WriteFile(file, []byte("some content"), 0644)
	c.Assert(err, IsNil)

	err = os.Chdir(tmp)
	c.Assert(err, IsNil)

	err = testutils.CreateZip(zipFile, filepath.Dir(file))
	c.Assert(err, IsNil)

	zr, err := zip.OpenReader(zipFile)
	c.Assert(err, IsNil)

	for _, zf := range zr.File {
		f, err := zf.Open()
		c.Assert(err, IsNil)
		defer func() { _ = f.Close() }()

		buf := make([]byte, 15)
		n, err := f.Read(buf)
		c.Assert(err, Equals, io.EOF)
		c.Check(n, Equals, 12)
		c.Check(buf[:n], DeepEquals, []byte("some content"))
	}
}
