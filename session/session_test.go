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

package session_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/session"
)

const (
	MySha256 = "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950"
)

func Test(t *testing.T) { TestingT(t) }

type sessionSuite struct{}

func (t *sessionSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&sessionSuite{})

func (t *sessionSuite) TestNewSession(c *C) {
	id_restorer := session.MockMakeSessionId(func() string {
		return "6ba7b8109dad11d180b400c04fd430c8"
	})
	defer id_restorer()

	rs_restorer := session.MockRandomString(func(int) string {
		return "1ItfzwGBeJ8wsJdP0Nlx"
	})
	defer rs_restorer()

	before := time.Now()
	tmp := c.MkDir()
	s := session.New(tmp, true)
	after := time.Now()

	defer s.Discard()

	c.Assert(s.Id, Equals, "6ba7b8109dad11d180b400c04fd430c8")
	c.Assert(s.Pw, Equals, "1ItfzwGBeJ8wsJdP0Nlx")
	c.Assert(s.Start.After(before) || s.Start.Equal(before), Equals, true)
	c.Assert(s.Start.Before(after) || s.Start.Equal(after), Equals, true)
	c.Assert(s.End.Equal(time.Time{}), Equals, true)
	c.Assert(s, Equals, session.GetSession(s.Id))
}

func (t *sessionSuite) TestRandomString(c *C) {
	for _, n := range []int{0, 1, 10, 20} {
		x := session.RandomString(n)
		y := session.RandomString(n)
		c.Assert(len(x), Equals, n)
		c.Assert(len(y), Equals, n)
		c.Assert(x == y, Equals, n == 0) // only empty strings are equal
	}
}

func (t *sessionSuite) TestDiscardSession(c *C) {
	s := session.New("", true)
	defer s.Discard()

	c.Assert(s, Equals, session.GetSession(s.Id))

	s.Discard()

	s = session.GetSession(s.Id)
	c.Assert(s, IsNil)
}

func (t *sessionSuite) TestCheckAuth(c *C) {
	s := session.New("", true)
	defer s.Discard()

	c.Assert(session.CheckAuth("foo", "bar"), Equals, false)
	c.Assert(session.CheckAuth(s.Id, s.Pw), Equals, true)
}

func (t *sessionSuite) TestAddMetadata(c *C) {
	s := session.New("", true)
	defer s.Discard()

	h, _ := metadata.NewSha256Digest(MySha256)
	c.Assert(s.A, HasLen, 0)
	c.Assert(s.HasArtefact(h), Equals, false)

	a := metadata.NewArtefact()
	a.Metadata.Name = "test-metadata"
	a.Metadata.Sha256 = h

	s.AddArtefact(a)

	c.Assert(s.A[h].Metadata.Name, Equals, "test-metadata")
	c.Assert(s.HasArtefact(h), Equals, true)
}

func (t *sessionSuite) TestAddDownload(c *C) {
	s := session.New("", true)
	defer s.Discard()

	h, _ := metadata.NewSha256Digest(MySha256)
	a := metadata.NewArtefact()
	a.Metadata.Name = "test-metadata"
	a.Metadata.Sha256 = h

	s.AddArtefact(a)

	di := metadata.Download{URL: "https://foo.bar", Sha256: h}
	s.AddDownload(di)
	c.Assert(s.A[h].Downloads[0].URL, Equals, "https://foo.bar")

	di.URL = "https://another/url"
	s.AddDownload(di)
	c.Assert(s.A[h].Downloads[0].URL, Equals, "https://foo.bar")
	c.Assert(s.A[h].Downloads[1].URL, Equals, "https://another/url")
}

func (t *sessionSuite) TestAddInvalidDownload(c *C) {
	s := session.New("", true)
	defer s.Discard()

	h, _ := metadata.NewSha256Digest(MySha256)
	a := metadata.NewArtefact()
	a.Metadata.Name = "test-metadata"
	a.Metadata.Sha256 = h

	s.AddArtefact(a)

	// adding an invalid sha1 must not crash the server
	di := metadata.Download{URL: "https://foo.bar", Sha256: h}
	s.AddDownload(di)
}

func (t *sessionSuite) TestSaveData(c *C) {
	s := session.New("", true)
	defer s.Discard()

	tmp := c.MkDir()
	tempfile := filepath.Join(tmp, "tempfile")

	h, _ := metadata.NewSha256Digest(MySha256)
	a := metadata.NewArtefact()
	a.AssetDir = tmp
	a.Tempfile = tempfile
	a.Metadata.Name = "test-metadata"
	a.Metadata.Sha256 = h

	s.AddArtefact(a)

	content := []byte("hello world")
	err := ioutil.WriteFile(tempfile, content, 0644)
	c.Assert(err, IsNil)

	err = s.SaveData(h)
	c.Assert(err, IsNil)

	// data is stored in file named after the digest value
	data, err := ioutil.ReadFile(filepath.Join(tmp, "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950.data"))
	c.Assert(err, IsNil)
	c.Assert(data, DeepEquals, []byte("hello world"))

	// see if temporary file deleted
	_, err = os.Stat(tempfile)
	c.Assert(errors.Is(err, os.ErrNotExist), Equals, true)
}

func (t *sessionSuite) TestSaveMetadata(c *C) {
	s := session.New("", true)
	defer s.Discard()

	tmp := c.MkDir()

	h, _ := metadata.NewSha256Digest(MySha256)
	a := metadata.NewArtefact()
	a.AssetDir = tmp
	a.Metadata.Name = "test-metadata"
	a.Metadata.Sha256 = h

	s.AddArtefact(a)

	err := s.SaveMetadata(h)
	c.Assert(err, IsNil)

	data, err := ioutil.ReadFile(filepath.Join(tmp, "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950.json"))
	c.Assert(err, IsNil)

	var j metadata.Artefact
	err = json.Unmarshal(data, &j)
	c.Assert(err, IsNil)
	c.Assert(j.Metadata.Name, Equals, "test-metadata")
}

func (t *sessionSuite) TestGetSession(c *C) {
	m := session.GetSession("invalid-session-id")
	c.Assert(m, IsNil)

	s := session.New("", true)
	defer s.Discard()

	m = session.GetSession(s.Id)
	c.Assert(m, Equals, s)
}
