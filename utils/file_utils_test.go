// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2025 Canonical Ltd.
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
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/utils"
)

func Test(t *testing.T) { TestingT(t) }

type fileutilsSuite struct {
	sl logger.Logger
}

var _ = Suite(&fileutilsSuite{logger.NewSessionLogger("test")})

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

	data := buf.Bytes()
	dest := bytes.NewReader(data)
	size := int64(len(data))
	c.Check(utils.ZipMatches(dest, size, []*regexp.Regexp{regexp.MustCompile(`^.*\.txt$`)}), Equals, true)
	c.Check(utils.ZipMatches(dest, size, []*regexp.Regexp{regexp.MustCompile(`^` + tmp)}), Equals, true)
	c.Check(utils.ZipMatches(dest, size, []*regexp.Regexp{regexp.MustCompile(`stuff/foo.txt$`)}), Equals, true)
	c.Check(utils.ZipMatches(dest, size, []*regexp.Regexp{regexp.MustCompile(`stuff/bar.txt$`)}), Equals, true)
	c.Check(utils.ZipMatches(dest, size, []*regexp.Regexp{regexp.MustCompile(`/bar.txt$`)}), Equals, true)
	c.Check(utils.ZipMatches(dest, size, []*regexp.Regexp{regexp.MustCompile(`baz.txt`)}), Equals, false)
	c.Check(utils.ZipMatches(dest, size, []*regexp.Regexp{regexp.MustCompile(`/b.*\.txt`), regexp.MustCompile(`/f.*\.txt`)}), Equals, true)
	c.Check(utils.ZipMatches(dest, size, []*regexp.Regexp{regexp.MustCompile(`/b*.txt`), regexp.MustCompile(`/z*.txt`)}), Equals, false)
}

func (t *fileutilsSuite) TestMoveFileRename(c *C) {
	restorer := utils.MockOsRename(func(newp, oldp string) error {
		err := os.Rename(newp, oldp)
		c.Assert(err, IsNil)
		return nil
	})
	defer restorer()

	tmp := c.MkDir()
	oldpath := filepath.Join(tmp, "oldfile")
	newpath := filepath.Join(tmp, "newfile")
	err := os.WriteFile(oldpath, []byte("Lorem ipsum dolor sit amet\n"), 0640)
	c.Assert(err, IsNil)

	err = utils.MoveFile(oldpath, newpath)
	c.Assert(err, IsNil)

	fi, err := os.Stat(newpath)
	c.Assert(err, IsNil)
	c.Assert(int(fi.Mode()&0777), Equals, 0640)

	content, err := os.ReadFile(newpath)
	c.Assert(err, IsNil)
	c.Assert(content, DeepEquals, []byte("Lorem ipsum dolor sit amet\n"))

	_, err = os.Stat(oldpath)
	c.Assert(err, ErrorMatches, "stat.*no such file or directory")
}

func (t *fileutilsSuite) TestMoveFileCopy(c *C) {
	restorer := utils.MockOsRename(func(newp, oldp string) error {
		return errors.New("rename failed")
	})
	defer restorer()

	tmp := c.MkDir()
	oldpath := filepath.Join(tmp, "oldfile")
	newpath := filepath.Join(tmp, "newfile")
	err := os.WriteFile(oldpath, []byte("Lorem ipsum dolor sit amet\n"), 0640)
	c.Assert(err, IsNil)

	err = utils.MoveFile(oldpath, newpath)
	c.Assert(err, IsNil)

	fi, err := os.Stat(newpath)
	c.Assert(err, IsNil)
	c.Assert(int(fi.Mode()&0777), Equals, 0640)

	content, err := os.ReadFile(newpath)
	c.Assert(err, IsNil)
	c.Assert(content, DeepEquals, []byte("Lorem ipsum dolor sit amet\n"))

	_, err = os.Stat(oldpath)
	c.Assert(err, ErrorMatches, "stat.*no such file or directory")
}

func (t *fileutilsSuite) TestGetLicense(c *C) {
	for _, tc := range []struct {
		filename string
		license  string
		errMsg   string
	}{
		{"export_test.go", "GPL-3", ""},
		{"../go.mod", "UNKNOWN", ""},
		{"does-not-exist", "", ""},
	} {
		res, err := utils.GetLicense(tc.filename, t.sl)
		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Check(res, Equals, tc.license)
		} else {
			c.Assert(err, ErrorMatches, tc.errMsg)
		}
	}
}

func (t *fileutilsSuite) TestCheckLicenseFiles(c *C) {
	for _, tc := range []struct {
		files   []string
		license string
	}{
		{[]string{"does-not-exist", "export_test.go"}, "GPL-3"},
		{[]string{"does-not-exist"}, ""},
	} {
		res, err := utils.CheckLicenseFiles(tc.files, t.sl)
		c.Assert(err, IsNil)
		c.Check(res, Equals, tc.license)
	}
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
