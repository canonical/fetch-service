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

package files_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/files"
)

func Test(t *testing.T) { TestingT(t) }

type filesSuite struct {
}

func (t *filesSuite) SetUpTest(c *C) {
}

func (t *filesSuite) TearDownTest(c *C) {
}

var _ = Suite(&filesSuite{})

func (t *filesSuite) TestArtifactFile(c *C) {
	tmp := c.MkDir()
	testFile := filepath.Join(tmp, "foo.txt")
	err := os.WriteFile(testFile, []byte("Some content"), 0644)
	c.Assert(err, IsNil)

	f, err := files.OpenArtifactFile(testFile)
	c.Assert(err, IsNil)
	defer func() { _ = f.Close() }()

	c.Check(f.Len(), Equals, 12)

	b := make([]byte, 15)
	n, err := f.Read(b)
	c.Assert(err, IsNil)
	c.Check(n, Equals, 12)
	c.Check(b, DeepEquals, []byte{'S', 'o', 'm', 'e', ' ', 'c', 'o', 'n', 't', 'e', 'n', 't', 0x00, 0x00, 0x00})

	pos, err := f.Seek(0, io.SeekStart)
	c.Assert(err, IsNil)
	c.Check(pos, Equals, int64(0))

	b = make([]byte, 10)
	n, err = f.ReadAt(b, 5)
	c.Assert(err, Equals, io.EOF)
	c.Check(n, Equals, 7)
	c.Check(b, DeepEquals, []byte{'c', 'o', 'n', 't', 'e', 'n', 't', 0x00, 0x00, 0x00})
}
