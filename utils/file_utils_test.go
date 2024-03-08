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

package utils_test

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/utils"
)

func Test(t *testing.T) { TestingT(t) }

type fileutilsSuite struct{}

var _ = Suite(&fileutilsSuite{})

func (t *fileutilsSuite) TestZipMatches(c *C) {
	tmp := c.MkDir()
	content := make([]byte, 5000)
	for i := range content {
		content[i] = byte(i & 0xff)
	}

	src := filepath.Join(tmp, "stuff")

	err := os.Mkdir(src, 0755)
	c.Assert(err, IsNil)

	err = os.WriteFile(filepath.Join(src, "foo.txt"), content, 0644)
	c.Assert(err, IsNil)

	err = os.WriteFile(filepath.Join(src, "bar.txt"), content, 0644)
	c.Assert(err, IsNil)

	var buf bytes.Buffer
	err = createZip(src, &buf)
	c.Assert(err, IsNil)

	dest := buf.Bytes()
	c.Check(utils.ZipMatches(dest, `^.*\.txt$`), Equals, true)
	c.Check(utils.ZipMatches(dest, `^`+tmp), Equals, true)
	c.Check(utils.ZipMatches(dest, `stuff/foo.txt$`), Equals, true)
	c.Check(utils.ZipMatches(dest, `stuff/bar.txt$`), Equals, true)
	c.Check(utils.ZipMatches(dest, `/bar.txt$`), Equals, true)
	c.Check(utils.ZipMatches(dest, `baz.txt`), Equals, false)
	c.Check(utils.ZipMatches(dest, `/b.*\.txt`, `/f.*\.txt`), Equals, true)
	c.Check(utils.ZipMatches(dest, `/b*.txt`, `/z*.txt`), Equals, false)
}

func createZip(src string, dest io.Writer) error {
	z := zip.NewWriter(dest)
	defer z.Close()

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		inf, err := os.Open(path)
		if err != nil {
			return err
		}
		defer inf.Close()

		outf, err := z.Create(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(outf, inf)
		if err != nil {
			return err
		}

		return nil
	})
}
