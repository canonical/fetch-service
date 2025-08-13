// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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

package git_test

import (
	"bytes"

	"github.com/gabriel-vasile/mimetype"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/git"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
)

type protocolSuite struct {
	slog logger.Logger
}

var _ = Suite(&protocolSuite{logger.NewSessionLogger("test")})

func (t *protocolSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

func (s *protocolSuite) TestGetGitProtocol(c *C) {
	a := fakeGitArtifact()
	proto := git.GetGitProtocol(a)
	c.Assert(proto, Equals, "version=2")

}

func (s *protocolSuite) TestGetGitProtocolFail(c *C) {
	a := metadata.NewArtifact() // not a git artifact
	proto := git.GetGitProtocol(a)
	c.Assert(proto, Equals, "")
}

func (s *protocolSuite) TestDecodeGitProtocolFail(c *C) {
	protocolDecodeError := "decode error"
	parseIntError := "strconv.ParseUint: parsing \"0\\x00\\x00\\x00\": invalid syntax"
	protocolShortMessage := "cannot read 5 bytes from input: EOF"
	parseUintError := `strconv.ParseUint: parsing "xxxx": invalid syntax`
	shortReadError := "cannot read 4 bytes from input: EOF"

	for _, tc := range []struct {
		data   []byte
		msgs   []string
		errmsg string
	}{
		{nil, []string{}, protocolDecodeError},                                  // nil input
		{[]byte(""), []string{}, protocolDecodeError},                           // empty input
		{[]byte("0"), []string{}, parseIntError},                                // invalid short input
		{[]byte("0000"), []string{}, ""},                                        // valid flush
		{[]byte("0001"), []string{}, shortReadError},                            // valid delimiter, missing rest of message
		{[]byte("0002"), []string{}, ""},                                        // valid finalizer
		{[]byte("0003"), []string{}, protocolDecodeError},                       // invalid error
		{[]byte("0004"), []string{}, protocolDecodeError},                       // invalid error
		{[]byte("0005"), []string{}, protocolShortMessage},                      // missing message
		{[]byte("xxxx"), []string{}, parseUintError},                            // invalid size
		{[]byte("0003foo"), []string{}, protocolDecodeError},                    // missing finalizer
		{[]byte("0007foo0000"), []string{"foo"}, ""},                            // valid message with finalizer
		{[]byte("0007foo0005!0000"), []string{"foo", "!"}, ""},                  // valid message with finalizer
		{[]byte("0007foo000dpackfile\nstuff"), []string{"foo", "packfile"}, ""}, // end at packfile
	} {
		msgs, err := git.DecodeGitProtocol(bytes.NewReader(tc.data), s.slog)
		if tc.errmsg == "" {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err.Error(), Equals, tc.errmsg)
		}
		c.Assert(msgs, DeepEquals, tc.msgs)
	}
}

func fakeGitArtifact() *metadata.Artifact {
	a := metadata.NewArtifact()
	a.CurrentDownload.RequestHeader = map[string][]string{
		"Accept":          []string{"application/x-git-upload-pack-result"},
		"Accept-Encoding": []string{"deflate, gzip, br, zstd"},
		"Content-Length":  []string{"175"},
		"Content-Type":    []string{"application/x-git-upload-pack-request"},
		"Git-Protocol":    []string{"version=2"},
		"User-Agent":      []string{"git/2.34.1"},
	}
	a.CurrentDownload.ResponseHeader = map[string][]string{
		"Content-Type": []string{"application/x-git-upload-pack-advertisement"},
	}
	a.MimeType = mimetype.Lookup("text/plain")

	return a
}

func fakeGitArtifactUnsuportedProtocol() *metadata.Artifact {
	a := metadata.NewArtifact()
	a.CurrentDownload.RequestHeader = map[string][]string{
		"Accept":          []string{"application/x-git-upload-pack-result"},
		"Accept-Encoding": []string{"deflate, gzip, br, zstd"},
		"Content-Length":  []string{"175"},
		"Content-Type":    []string{"application/x-git-upload-pack-request"},
		"Git-Protocol":    []string{"version=1"},
		"User-Agent":      []string{"git/2.34.1"},
	}
	a.CurrentDownload.ResponseHeader = map[string][]string{
		"Content-Type": []string{"application/x-git-upload-pack-advertisement"},
	}
	a.MimeType = mimetype.Lookup("text/plain")

	return a
}

func fakeGitArtifactNoProtocolVersion() *metadata.Artifact {
	a := metadata.NewArtifact()
	a.CurrentDownload.RequestHeader = map[string][]string{
		"Accept":          []string{"application/x-git-upload-pack-result"},
		"Accept-Encoding": []string{"deflate, gzip, br, zstd"},
		"Content-Length":  []string{"175"},
		"Content-Type":    []string{"application/x-git-upload-pack-request"},
		"User-Agent":      []string{"git/2.34.1"},
	}
	a.CurrentDownload.ResponseHeader = map[string][]string{
		"Content-Type": []string{"application/x-git-upload-pack-advertisement"},
	}
	a.MimeType = mimetype.Lookup("text/plain")

	return a
}
