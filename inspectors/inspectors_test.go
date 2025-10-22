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

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
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
	a := metadata.NewArtifact()

	s := session.New(c.MkDir(), 0, false, nil)
	defer s.Discard()

	err := s.Insps.RunRequestInspectors(a)
	c.Assert(err, IsNil)
	c.Assert(len(a.RequestInspection), Equals, 1)
	c.Assert(a.RequestInspection["default"], DeepEquals, &Inspection{
		Opinion: opinions.Unknown,
		Reason:  "the request was not recognized by any format inspector",
	})
}

func (t *inspectorsSuite) TestRunRequestInspectorsPermissive(c *C) {
	a := metadata.NewArtifact()

	s := session.New(c.MkDir(), 0, true, nil)
	defer s.Discard()

	err := s.Insps.RunRequestInspectors(a)
	c.Assert(err, IsNil)
	c.Assert(len(a.RequestInspection), Equals, 1)
	c.Assert(a.RequestInspection["default"], DeepEquals, &Inspection{
		Opinion: opinions.Unknown,
		Reason:  "the request was not recognized by any format inspector",
	})
}

func (t *inspectorsSuite) TestRunArtifactInspectors(c *C) {
	dir := c.MkDir()
	data := []byte("Measure twice, saw once.\n")
	err := os.WriteFile(filepath.Join(dir, "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950.data"), data, 0644)
	c.Assert(err, IsNil)

	h, _ := digests.NewSha256Digest(MySha256)
	a := metadata.NewArtifact()
	a.CurrentDownload.ContentType = "text/plain"
	a.CurrentDownload.Sha256 = h
	a.Metadata.Sha256 = h

	s := session.New(c.MkDir(), 0, false, nil)
	defer s.Discard()

	err = s.Insps.RunArtifactInspectors(dir, a)
	c.Assert(err, Equals, nil)
	c.Assert(a.Metadata.Type, Equals, "text/plain; charset=utf-8")
	c.Assert(len(a.ResponseInspection), Equals, 1)
	c.Assert(a.ResponseInspection["default"], DeepEquals, &Inspection{
		Opinion: opinions.Unknown,
		Reason:  "the artifact file content was not recognized by any format inspector",
	})
	c.Assert(a.Rejected(), Equals, true)
}

func (t *inspectorsSuite) TestRunArtifactInspectorsPermissive(c *C) {
	dir := c.MkDir()
	data := []byte("Measure twice, saw once.\n")
	err := os.WriteFile(filepath.Join(dir, "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950.data"), data, 0644)
	c.Assert(err, IsNil)

	h, _ := digests.NewSha256Digest(MySha256)
	a := metadata.NewArtifact()
	a.CurrentDownload.ContentType = "text/plain"
	a.CurrentDownload.Sha256 = h
	a.CurrentDownload.URL = "http://some.url"
	a.Metadata.Sha256 = h

	s := session.New(dir, 0, true, nil)
	defer s.Discard()

	err = s.Insps.RunArtifactInspectors(dir, a)
	c.Assert(err, IsNil)
	c.Assert(a.Metadata.Type, Equals, "text/plain; charset=utf-8")
	c.Assert(len(a.ResponseInspection), Equals, 1)
	c.Assert(a.ResponseInspection["default"], DeepEquals, &Inspection{
		Opinion: opinions.Unknown,
		Reason:  "the artifact file content was not recognized by any format inspector",
	})
	c.Assert(a.Rejected(), Equals, true)
}
