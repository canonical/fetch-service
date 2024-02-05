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
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/session"
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

func (t *inspectorsSuite) TestRunRequestInspectors(c *C) {
	a := metadata.NewArtefact()

	s := session.New(false)
	defer s.Discard()

	err := s.Insps.RunRequestInspectors(a)
	c.Assert(err, IsNil)
	c.Assert(a.RequestInspection, DeepEquals, metadata.InspectionMap{
		"default": &metadata.Inspection{
			Opinion: metadata.Rejected,
			Reason:  "no further inspection pending",
		},
	})
}

func (t *inspectorsSuite) TestRunRequestInspectorsPermissive(c *C) {
	a := metadata.NewArtefact()

	s := session.New(true)
	defer s.Discard()

	err := s.Insps.RunRequestInspectors(a)
	c.Assert(err, IsNil)
	c.Assert(a.RequestInspection, DeepEquals, metadata.InspectionMap{
		"default": &metadata.Inspection{
			Opinion: metadata.Pending,
			Reason:  "allowed because running in permissive mode",
		},
	})
}

func (t *inspectorsSuite) TestRunArtefactInspectors(c *C) {
	dir := c.MkDir()
	data := []byte("Measure twice, saw once.\n")
	err := os.WriteFile(filepath.Join(dir, "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950.data"), data, 0644)
	c.Assert(err, IsNil)

	h, _ := metadata.NewSha256Digest(MySha256)
	a := metadata.NewArtefact()
	a.CurrentDownload.ContentType = "text/plain"
	a.CurrentDownload.Sha256 = h
	a.Metadata.Sha256 = h

	s := session.New(false)
	defer s.Discard()

	err = s.Insps.RunArtefactInspectors(dir, a)
	c.Assert(err, Equals, ErrRejectedArtefact)
	c.Assert(a.Metadata.Type, Equals, "text/plain; charset=utf-8")
	c.Assert(a.ResponseInspection, DeepEquals, metadata.InspectionMap{
		"default": &metadata.Inspection{
			Opinion: metadata.Rejected,
			Reason:  "artefact format unknown",
		},
	})
	c.Assert(a.State, Equals, metadata.InspectionState("Rejected"))
}

func (t *inspectorsSuite) TestRunArtefactInspectorsPermissive(c *C) {
	dir := c.MkDir()
	data := []byte("Measure twice, saw once.\n")
	err := os.WriteFile(filepath.Join(dir, "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950.data"), data, 0644)
	c.Assert(err, IsNil)

	h, _ := metadata.NewSha256Digest(MySha256)
	a := metadata.NewArtefact()
	a.CurrentDownload.ContentType = "text/plain"
	a.CurrentDownload.Sha256 = h
	a.Metadata.Sha256 = h

	s := session.New(true)
	defer s.Discard()

	err = s.Insps.RunArtefactInspectors(dir, a)
	c.Assert(err, IsNil)
	c.Assert(a.Metadata.Type, Equals, "text/plain; charset=utf-8")
	c.Assert(a.ResponseInspection, DeepEquals, metadata.InspectionMap{
		"default": &metadata.Inspection{
			Opinion: metadata.Rejected,
			Reason:  "artefact format unknown",
		},
	})
	c.Assert(a.State, Equals, metadata.InspectionState("Rejected"))
}

func (t *inspectorsSuite) TestDefaultInspectorRequest(c *C) {
	a := metadata.NewArtefact()

	var iface inspectors.Inspector
	ins := inspectors.DefaultInspector{}
	c.Assert(ins, Implements, &iface)

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	c.Assert(a.Rejected(), Equals, true)
	c.Assert(a.RequestInspection["default"], DeepEquals,
		&metadata.Inspection{
			Opinion: metadata.Rejected,
			Reason:  "no further inspection pending",
		},
	)
}

func (t *inspectorsSuite) TestDefaultInspectorRequestPermissive(c *C) {
	a := metadata.NewArtefact()

	var iface inspectors.Inspector
	ins := inspectors.DefaultInspector{Permissive: true}
	c.Assert(ins, Implements, &iface)

	err := ins.InspectRequest(a)
	c.Assert(err, IsNil)
	c.Assert(a.Pending(), Equals, true)
	c.Assert(a.RequestInspection["default"], DeepEquals,
		&metadata.Inspection{
			Opinion: metadata.Pending,
			Reason:  "allowed because running in permissive mode",
		},
	)
}

func (t *inspectorsSuite) TestDefaultInspectorResponse(c *C) {
	a := metadata.NewArtefact()
	a.Metadata.Type = "application/unit-test"
	a.State = metadata.ResponseState

	var iface inspectors.Inspector
	ins := inspectors.DefaultInspector{}
	c.Assert(ins, Implements, &iface)

	err := ins.InspectArtefact(nil, a)
	c.Assert(err, IsNil)
	c.Assert(a.Rejected(), Equals, true)
	c.Assert(a.ResponseInspection["default"], DeepEquals,
		&metadata.Inspection{
			Opinion: metadata.Rejected,
			Reason:  "artefact format unknown",
		},
	)
}

func (t *inspectorsSuite) TestDefaultInspectorResponsePermissive(c *C) {
	a := metadata.NewArtefact()
	a.Metadata.Type = "application/unit-test"
	a.State = metadata.ResponseState

	var iface inspectors.Inspector
	ins := inspectors.DefaultInspector{}
	ins.Permissive = true
	c.Assert(ins, Implements, &iface)

	err := ins.InspectArtefact(nil, a)
	c.Assert(err, IsNil)
	c.Assert(a.Rejected(), Equals, true)
	c.Assert(a.ResponseInspection["default"], DeepEquals,
		&metadata.Inspection{
			Opinion: metadata.Rejected,
			Reason:  "artefact format unknown",
		},
	)
}
