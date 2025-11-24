// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2024 Canonical Ltd.
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
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/secrets"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/session"
	"github.com/canonical/fetch-service/version"
)

const (
	MySha256 = "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950"
)

func Test(t *testing.T) { TestingT(t) }

type sessionSuite struct{}

var _ = Suite(&sessionSuite{})

func (t *sessionSuite) TestNewSession(c *C) {
	idRestorer := session.MockMakeSessionID(func() string {
		return "6ba7b8109dad11d180b400c04fd430c8"
	})
	defer idRestorer()

	rsRestorer := session.MockRandomString(func(int) string {
		return "1ItfzwGBeJ8wsJdP0Nlx"
	})
	defer rsRestorer()

	before := time.Now()
	tmp := c.MkDir()
	s := session.New(tmp, 0, true, nil, config.OverrideInspectorsConfig{})
	after := time.Now()

	defer s.Discard()

	c.Assert(s.ID, Equals, "6ba7b8109dad11d180b400c04fd430c8")
	c.Assert(s.Token, Equals, "1ItfzwGBeJ8wsJdP0Nlx")
	c.Assert(s.Start.After(before) || s.Start.Equal(before), Equals, true)
	c.Assert(s.Start.Before(after) || s.Start.Equal(after), Equals, true)
	c.Assert(s.End.Equal(time.Time{}), Equals, true)
	c.Assert(s, Equals, session.GetSession(s.ID))
	c.Assert(len(s.Secrets), Equals, 0)
}

func (t *sessionSuite) TestNewWithID(c *C) {
	restorer := session.MockMakeSessionID(func() string {
		return "6ba7b8109dad11d180b400c04fd430c8"
	})
	defer restorer()

	tmp := c.MkDir()
	s := session.NewWithID("known-session-id", "known-token", tmp, 0, true, nil, config.OverrideInspectorsConfig{})
	defer s.Discard()
	c.Assert(s.ID, Equals, "known-session-id")
	c.Assert(s.Token, Equals, "known-token")
	c.Assert(len(s.Secrets), Equals, 0)

	// Re-create session with same ID
	s = session.NewWithID("known-session-id", "known-token", tmp, 0, true, nil, config.OverrideInspectorsConfig{})
	defer s.Discard()
	c.Assert(s.ID, Equals, "6ba7b8109dad11d180b400c04fd430c8")
	c.Assert(s.Token, Equals, "known-token")
	c.Assert(len(s.Secrets), Equals, 0)
}

func (t *sessionSuite) TestNewWithSecrets(c *C) {
	tmp := c.MkDir()
	sec := []secrets.Secret{
		{Type: secrets.BasicAuthType, URL: glob.MustCompile("http://www.example.com/*")},
	}
	s := session.NewWithID("known-session-id", "known-token", tmp, 0, true, sec, config.OverrideInspectorsConfig{})
	defer s.Discard()
	c.Assert(s.ID, Equals, "known-session-id")
	c.Assert(s.Token, Equals, "known-token")
	c.Assert(s.Secrets, DeepEquals, sec)
}

func (t *sessionSuite) TestRandomString(c *C) {
	for _, n := range []int{0, 10, 20} {
		x := session.RandomString(n)
		y := session.RandomString(n)
		c.Assert(len(x), Equals, n)
		c.Assert(len(y), Equals, n)
		c.Assert(x == y, Equals, n == 0) // only empty strings are equal
	}
}

func (t *sessionSuite) TestDiscardSession(c *C) {
	s := session.New("", 0, true, nil, config.OverrideInspectorsConfig{})
	defer s.Discard()

	c.Assert(s, Equals, session.GetSession(s.ID))

	s.Discard()

	s = session.GetSession(s.ID)
	c.Assert(s, IsNil)
}

func (t *sessionSuite) TestSessionTimeout(c *C) {
	s1 := session.New("", 1*time.Second, true, nil, config.OverrideInspectorsConfig{})
	defer s1.Discard()

	time.Sleep(500 * time.Millisecond)

	s2 := session.New("", 1*time.Second, true, nil, config.OverrideInspectorsConfig{})
	defer s2.Discard()

	time.Sleep(2 * time.Second)
	sessionID := <-session.ExpiredSessionID
	c.Assert(sessionID, Equals, s1.ID)
	sessionID = <-session.ExpiredSessionID
	c.Assert(sessionID, Equals, s2.ID)
}

func (t *sessionSuite) TestSessionTimeoutCancel(c *C) {
	s := session.New("", 2*time.Second, true, nil, config.OverrideInspectorsConfig{})
	time.Sleep(1 * time.Second)
	s.Discard()
	s = session.GetSession(s.ID)
	c.Assert(s, IsNil)
}

func (t *sessionSuite) TestHasArtifact(c *C) {
	for _, tc := range []struct {
		addToSession bool
	}{
		{true},
		{false},
	} {
		s := session.New("", 0, true, nil, config.OverrideInspectorsConfig{})
		defer s.Discard()

		digest, err := digests.NewSha256Digest(MySha256)
		c.Assert(err, IsNil)

		a := metadata.NewArtifact()
		a.Metadata.Sha256 = digest
		if tc.addToSession {
			s.AddArtifact(a)
		}

		hasArtifact := tc.addToSession

		c.Assert(s.HasArtifact(digest), Equals, hasArtifact)
	}
}

func (t *sessionSuite) TestArtifactResult(c *C) {
	for _, tc := range []struct {
		addToSession bool
		result       opinions.OpinionKind
	}{
		{true, opinions.Approved},
		{true, opinions.Rejected},
		{false, opinions.Rejected},
	} {
		s := session.New("", 0, true, nil, config.OverrideInspectorsConfig{})
		defer s.Discard()

		digest, err := digests.NewSha256Digest(MySha256)
		c.Assert(err, IsNil)

		a := metadata.NewArtifact()
		a.Metadata.Sha256 = digest
		a.Result = tc.result
		if tc.addToSession {
			s.AddArtifact(a)
		}

		c.Assert(s.ArtifactResult(digest), Equals, tc.result)
	}
}

func (t *sessionSuite) TestCheckAuth(c *C) {
	s := session.New("", 0, true, nil, config.OverrideInspectorsConfig{})
	defer s.Discard()

	c.Assert(session.CheckAuth("foo", "bar"), Equals, false)
	c.Assert(session.CheckAuth(s.ID, s.Token), Equals, true)
}

func (t *sessionSuite) TestAddMetadata(c *C) {
	s := session.New("", 0, true, nil, config.OverrideInspectorsConfig{})
	defer s.Discard()

	h, err := digests.NewSha256Digest(MySha256)
	c.Assert(err, IsNil)
	c.Assert(s.A, HasLen, 0)
	c.Assert(s.HasArtifact(h), Equals, false)

	a := metadata.NewArtifact()
	a.Metadata.Name = "test-metadata"
	a.Metadata.Sha256 = h

	s.AddArtifact(a)

	c.Assert(s.A[h].Metadata.Name, Equals, "test-metadata")
	c.Assert(s.HasArtifact(h), Equals, true)
}

func (t *sessionSuite) TestAddDownload(c *C) {
	s := session.New("", 0, true, nil, config.OverrideInspectorsConfig{})
	defer s.Discard()

	h, err := digests.NewSha256Digest(MySha256)
	c.Assert(err, IsNil)
	a := metadata.NewArtifact()
	a.Metadata.Name = "test-metadata"
	a.Metadata.Sha256 = h

	s.AddArtifact(a)

	di := metadata.Download{URL: "https://foo.bar", Sha256: h}
	s.AddDownload(di)
	c.Assert(s.A[h].Downloads[0].URL, Equals, "https://foo.bar")

	di.URL = "https://another/url"
	s.AddDownload(di)
	c.Assert(s.A[h].Downloads[0].URL, Equals, "https://foo.bar")
	c.Assert(s.A[h].Downloads[1].URL, Equals, "https://another/url")
}

func (t *sessionSuite) TestAddInvalidDownload(c *C) {
	s := session.New("", 0, true, nil, config.OverrideInspectorsConfig{})
	defer s.Discard()

	h, err := digests.NewSha256Digest(MySha256)
	c.Assert(err, IsNil)
	a := metadata.NewArtifact()
	a.Metadata.Name = "test-metadata"
	a.Metadata.Sha256 = h

	s.AddArtifact(a)

	// adding an invalid sha1 must not crash the server
	di := metadata.Download{URL: "https://foo.bar", Sha256: h}
	s.AddDownload(di)
}

type saveDataTest struct {
	artifactAdded bool   // Whether the artifact exists in the session
	mkdirFail     bool   // Whether spool directory creation fails
	errMsg        string // The expected error message, if any
}

var saveDataTests = []saveDataTest{{
	artifactAdded: true,
	mkdirFail:     false,
	errMsg:        "",
}, {
	artifactAdded: false,
	mkdirFail:     false,
	errMsg:        "",
}, {
	artifactAdded: true,
	mkdirFail:     true,
	errMsg:        "cannot create dir",
}}

func (t *sessionSuite) TestSaveData(c *C) {
	for _, tc := range saveDataTests {
		session.MockOsMkdirAll(func(path string, perm os.FileMode) error {
			if tc.mkdirFail {
				return errors.New("cannot create dir")
			}
			return os.MkdirAll(path, perm)
		})

		s := session.New("", 0, true, nil, config.OverrideInspectorsConfig{})
		defer s.Discard()

		tmp := c.MkDir()
		tempfile := filepath.Join(tmp, "tempfile")

		h, err := digests.NewSha256Digest(MySha256)
		c.Assert(err, IsNil)

		a := metadata.NewArtifact()
		a.AssetDir = tmp
		a.Tempfile = tempfile
		a.Metadata.Name = "test-metadata"
		a.Metadata.Sha256 = h

		// The temporary file exists for every downloaded artifact
		content := []byte("hello world")
		err = os.WriteFile(tempfile, content, 0644)
		c.Assert(err, IsNil)

		if tc.artifactAdded {
			s.AddArtifact(a)
		}

		err = s.SaveData(a)
		if tc.errMsg == "" {
			c.Assert(err, IsNil)

			// data is stored in file named after the digest value
			data, err := os.ReadFile(filepath.Join(tmp, "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950.data"))
			c.Assert(err, IsNil)
			c.Assert(data, DeepEquals, []byte("hello world"))
		} else {
			c.Assert(err, ErrorMatches, tc.errMsg)
		}

		// see if temporary file deleted
		_, err = os.Stat(tempfile)
		c.Assert(errors.Is(err, os.ErrNotExist), Equals, true)
	}
}

func (t *sessionSuite) TestSaveMetadata(c *C) {
	s := session.New("", 0, true, nil, config.OverrideInspectorsConfig{})
	defer s.Discard()

	tmp := c.MkDir()

	h, err := digests.NewSha256Digest(MySha256)
	c.Assert(err, IsNil)
	a := metadata.NewArtifact()
	a.AssetDir = tmp
	a.Metadata.Name = "test-metadata"
	a.Metadata.Sha256 = h

	s.AddArtifact(a)

	err = s.SaveMetadata(h)
	c.Assert(err, IsNil)

	data, err := os.ReadFile(filepath.Join(tmp, "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950.json"))
	c.Assert(err, IsNil)

	var j metadata.Artifact
	err = json.Unmarshal(data, &j)
	c.Assert(err, IsNil)
	c.Assert(j.Metadata.Name, Equals, "test-metadata")
}

func (t *sessionSuite) TestGetSession(c *C) {
	m := session.GetSession("invalid-session-id")
	c.Assert(m, IsNil)

	s := session.New("", 0, true, nil, config.OverrideInspectorsConfig{})
	defer s.Discard()

	m = session.GetSession(s.ID)
	c.Assert(m, Equals, s)
}

func (t *sessionSuite) TestSessionMetadata(c *C) {
	for _, tc := range []struct {
		permissive bool
		policy     string
	}{
		{true, "permissive"},
		{false, "strict"},
	} {
		s := session.New("", 0, tc.permissive, nil, config.OverrideInspectorsConfig{})
		defer s.Discard()

		m := s.Metadata()
		c.Check(m.Generator, Equals, fmt.Sprintf("fetch-service %s", version.Version))
		c.Check(m.Comment, Equals, "Metadata format is unstable and may change without prior notice.")
		c.Check(m.Policy, Equals, tc.policy)
		c.Check(m.SessionID, Equals, s.ID)
		c.Check(m.StartTime, Not(DeepEquals), time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC))
		c.Check(slices.Contains(m.Inspectors, "default"), Equals, true)
		c.Check(m.SpoolPath, Not(Equals), "")
	}
}

func (t *sessionSuite) TestFinish(c *C) {
	spool := c.MkDir()
	s := session.New(spool, 0, true, nil, config.OverrideInspectorsConfig{})
	defer s.Discard()

	sessionDir := filepath.Join(spool, s.ID)
	assetDir := filepath.Join(sessionDir, "assets")
	cacheDir := filepath.Join(sessionDir, "cache")

	s.SessionDir = sessionDir
	s.CacheDir = cacheDir

	err := os.MkdirAll(assetDir, 0755)
	c.Assert(err, IsNil)

	err = os.MkdirAll(cacheDir, 0755)
	c.Assert(err, IsNil)

	for _, tc := range []struct {
		digest      string
		mkdirFail   bool
		jsonCreated bool
		errMsg      string
	}{
		{"1234567890123456789012345678901234567890123456789012345678901234", false, true, ""},
		{"1234567890123456789012345678901234567890123456789012345678901234", true, true, "cannot create dir"},
		{"invalid-digest", false, false, ""},
	} {

		session.MockOsMkdirAll(func(path string, perm os.FileMode) error {
			if tc.mkdirFail {
				return errors.New("cannot create dir")
			}
			return os.MkdirAll(path, perm)
		})

		a := metadata.Artifact{AssetDir: assetDir}
		d, err := digests.NewSha256Digest(tc.digest)
		if err == nil {
			a.Metadata.Sha256 = d
		}
		s.A[d] = &a

		err = s.Finish()
		if tc.errMsg == "" {
			c.Assert(err, IsNil)

			// Finishing again shouldn't result in error
			err = s.Finish()
			c.Assert(err, IsNil)
		} else {
			c.Assert(err, ErrorMatches, tc.errMsg)
		}

		// Check if metadata files were created
		_, err = os.Stat(filepath.Join(assetDir, tc.digest+".json"))
		if tc.jsonCreated {
			c.Check(err, IsNil)
		} else {
			c.Check(err, ErrorMatches, "stat .*: no such file or directory")
		}

	}

	_, statErr := os.Stat(s.CacheDir)
	c.Assert(os.IsNotExist(statErr), Equals, true)
}

func (t *sessionSuite) TestRevokeToken(c *C) {
	for _, tc := range []struct {
		token  string
		result bool
	}{
		{"right-token", true},
		{"wrong-token", false},
	} {
		spool := c.MkDir()
		s := session.New(spool, 0, true, nil, config.OverrideInspectorsConfig{})
		defer s.Discard()

		s.Token = "right-token"

		res := s.Revoke(tc.token)
		c.Check(res, Equals, tc.result)
	}
}

func (t *sessionSuite) TestIsRevoked(c *C) {
	for _, tc := range []struct {
		revoked bool
		result  bool
	}{
		{true, true},
		{false, false},
	} {
		spool := c.MkDir()
		s := session.New(spool, 0, true, nil, config.OverrideInspectorsConfig{})
		defer s.Discard()

		s.Token = "token"
		if tc.revoked {
			s.Revoke("token")
		}

		res := s.IsRevoked()
		c.Check(res, Equals, tc.result)
	}
}

func (t *sessionSuite) TestArtifacts(c *C) {
	spool := c.MkDir()
	s := session.New(spool, 0, true, nil, config.OverrideInspectorsConfig{})
	defer s.Discard()

	d0, err := digests.NewSha256Digest("1234567890123456789012345678901234567890123456789012345678901234")
	c.Assert(err, IsNil)
	d1, err := digests.NewSha256Digest("1111111111222222222233333333334444444444555555555566666666667777")
	c.Assert(err, IsNil)

	m0 := metadata.Artifact{}
	m1 := metadata.Artifact{}

	s.A[d0] = &m0
	s.A[d1] = &m1

	a := s.Artifacts()
	c.Check(len(a), Equals, 2)
	c.Check((a[0] == &m0 && a[1] == &m1) || (a[0] == &m1 && a[1] == &m0), Equals, true)
}

func (t *sessionSuite) TestSize(c *C) {
	spool := c.MkDir()
	s1 := session.New(spool, 0, true, nil, config.OverrideInspectorsConfig{})
	defer s1.Discard()

	s2 := session.New(spool, 0, true, nil, config.OverrideInspectorsConfig{})
	defer s2.Discard()

	n := session.Sessions.Size()
	c.Assert(n, Equals, 2)
}

func (t *sessionSuite) TestFinishAll(c *C) {
	spool := c.MkDir()
	s1 := session.New(spool, 0, true, nil, config.OverrideInspectorsConfig{})
	defer s1.Discard()

	s2 := session.New(spool, 0, true, nil, config.OverrideInspectorsConfig{})
	defer s2.Discard()

	n := session.Sessions.Size()
	c.Assert(n, Equals, 2)

	session.FinishAll()

	n = session.Sessions.Size()
	c.Assert(n, Equals, 0)
}

func (t *sessionSuite) TestNumSessions(c *C) {
	spool := c.MkDir()
	s1 := session.New(spool, 0, true, nil, config.OverrideInspectorsConfig{})
	defer s1.Discard()

	s2 := session.New(spool, 0, false, nil, config.OverrideInspectorsConfig{})
	defer s2.Discard()

	n := session.NumSessions()
	c.Assert(n, Equals, 2)
}

func (t *sessionSuite) TestSessionInfos(c *C) {
	spool := c.MkDir()
	s1 := session.New(spool, 0, true, nil, config.OverrideInspectorsConfig{})
	defer s1.Discard()

	s2 := session.New(spool, 0, false, nil, config.OverrideInspectorsConfig{})
	defer s2.Discard()

	all := session.SessionInfos()
	c.Assert((all[0].SessionID == s1.ID && all[1].SessionID == s2.ID) ||
		(all[0].SessionID == s2.ID && all[1].SessionID == s1.ID), Equals, true)
}
