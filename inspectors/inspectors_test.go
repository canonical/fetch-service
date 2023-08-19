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

package inspectors_test

import (
	"os"
	"path/filepath"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
)

const (
	MySha256 = "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950"
)

func Test(t *testing.T) { TestingT(t) }

type inspectorsSuite struct{}

func (t *inspectorsSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&inspectorsSuite{})

func (s *inspectorsSuite) TestRunInspectors(c *C) {
	//ctx := metadata.NewInspectionContext()

	dir := c.MkDir()
	data := []byte("Measure twice, saw once.\n")
	err := os.WriteFile(filepath.Join(dir, "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950.data"), data, 0644)
	c.Assert(err, IsNil)

	h, _ := metadata.NewSha256Digest(MySha256)
	md := &metadata.Metadata{Sha256: h}
	di := &metadata.DownloadInfo{ContentType: "text/plain", Sha256: h}
	insps := inspectors.New()

	ch := make(chan interface{})

	err = insps.RunInspectors(dir, md, di, ch)
	c.Assert(err, IsNil)
	c.Assert(md.Type, Equals, "text/plain; charset=utf-8")

	// TODO: improve this test to see if registered inspectors ran as expected
}

func (s *inspectorsSuite) TestDefaultInspector(c *C) {
	md := &metadata.Metadata{Type: "application/unit-test"}
	di := &metadata.DownloadInfo{}

	var iface inspectors.Inspector
	ins := inspectors.DefaultInspector{}
	c.Assert(ins, Implements, &iface)

	ch := make(chan interface{})

	stop, err := ins.Inspect("any-filename", md, di, ch)
	c.Assert(err, IsNil)
	c.Assert(stop, Equals, true)
	c.Assert(md.Annotations, HasLen, 1)
	c.Assert(md.Annotations["default.format.unknown"].Value, HasLen, 0)
}
