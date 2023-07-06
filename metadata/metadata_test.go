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

package metadata_test

import (
	"os"
	"path/filepath"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/metadata"
)

func Test(t *testing.T) { TestingT(t) }

type metadataSuite struct{}

var _ = Suite(&metadataSuite{})

func (s *metadataSuite) TestAnnotation(c *C) {
	md := metadata.Metadata{}

	md.Annotate(metadata.Notice, "test.notice", "annotation text")

	data := metadata.AnnotationDetails{"text": "test"}
	md.Annotate(metadata.PolicyViolation, "test.violation", "more annotation text").SetDetails(data)

	c.Assert(md.Annotations, HasLen, 2)
	c.Assert(md.Annotations["test.notice"].Kind, Equals, metadata.Notice)
	c.Assert(md.Annotations["test.notice"].Origin, Equals, "unknown")
	c.Assert(md.Annotations["test.notice"].Value, Equals, "annotation text")
	c.Assert(md.Annotations["test.notice"].Details, HasLen, 0)
	c.Assert(md.Annotations["test.violation"].Kind, Equals, metadata.PolicyViolation)
	c.Assert(md.Annotations["test.violation"].Origin, Equals, "unknown")
	c.Assert(md.Annotations["test.violation"].Value, Equals, "more annotation text")
	c.Assert(md.Annotations["test.violation"].Details, DeepEquals, metadata.AnnotationDetails{"text": "test"})
}

func (s *metadataSuite) TestRunInspectors(c *C) {
	ctx := metadata.NewInspectionContext()

	dir := c.MkDir()
	data := []byte("Measure twice, saw once.\n")
	err := os.WriteFile(filepath.Join(dir, "my-sha1-sum.bin"), data, 0644)
	c.Assert(err, IsNil)

	md := &metadata.Metadata{Sha1: "my-sha1-sum"}
	di := &metadata.DownloadInfo{ContentType: "text/plain", Sha1: "my-sha1-sum"}

	err = ctx.RunInspectors(dir, md, di)
	c.Assert(err, IsNil)
	c.Assert(md.Type, Equals, "text/plain; charset=utf-8")

	// TODO: improve this test to see if registered inspectors ran as expected
}

func (s *metadataSuite) TestDefaultInspector(c *C) {
	md := &metadata.Metadata{Type: "application/unit-test"}
	di := &metadata.DownloadInfo{}

	var iface metadata.Inspector
	ins := metadata.DefaultInspector{}
	c.Assert(ins, Implements, &iface)

	stop, err := ins.Inspect("any-filename", md, di, nil)
	c.Assert(err, IsNil)
	c.Assert(stop, Equals, true)
	c.Assert(md.Annotations, HasLen, 1)
	c.Assert(md.Annotations["default.unknown"].Kind, Equals, metadata.Warning)
	c.Assert(md.Annotations["default.unknown"].Origin, Equals, "metadata.defaultInspector")
	c.Assert(md.Annotations["default.unknown"].Value, Equals, "unknown file format")
	c.Assert(md.Annotations["default.unknown"].Details, HasLen, 0)
}
