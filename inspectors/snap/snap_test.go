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

package snap_test

import (
	"errors"
	"io"
	"testing"

	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/snap"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type snapSuite struct{}

var _ = Suite(&snapSuite{})

func Test(t *testing.T) { TestingT(t) }

func (s *snapSuite) TestSnapInspectorID(c *C) {
	ins := snap.NewSnapInspector()
	c.Assert(ins.ID(), Equals, "snap")
}

func (s *snapSuite) TestInspectRequest(c *C) {
	for _, tc := range []struct {
		url      string
		approved bool
	}{
		{"https://api.snapcraft.io:443/api/v1/snaps/download/foo_42.snap", true},
		{"https://x.snapcraftcontent.com:443/subdir/foo_42.snap?", true},
		{"https://api.snapcraft.io:443/v2/snaps/download/foo_42.snap", false},
		{"https://x.snapcraftcontent.com:443/subdir/foo_42.snap", false},
		{"https://api.snapcraft.io:443/v3/snaps/download/foo_42.snap", false},
		{"http://api.snapcraft.io/v2/snaps/download/foo_42.snap", false},
		{"https://x.snapcraftcontent.com:443/subdir/foo_42.snap", false},
		{"https://api.snapcraft.io/v2/snaps/info", false},
	} {
		ins := snap.NewSnapInspector()
		a := metadata.NewArtefact()
		a.CurrentDownload = metadata.Download{URL: tc.url}

		err := ins.InspectRequest(a)
		c.Assert(err, IsNil)

		insp, ok := a.RequestInspection[ins.ID()]
		c.Assert(ok, Equals, tc.approved)
		if tc.approved {
			c.Assert(insp.Opinion, Equals, opinions.Pending)
		}
	}
}

func (s *snapSuite) TestInspectRequestError(c *C) {
	ins := snap.NewSnapInspector()
	a := metadata.NewArtefact()
	a.CurrentDownload = metadata.Download{URL: "::"}

	err := ins.InspectRequest(a)
	c.Assert(err, ErrorMatches, ".*: missing protocol scheme")
}

func (s *snapSuite) TestSnapArtefactInspector(c *C) {
	a := metadata.NewArtefact()
	a.Metadata.Type = "application/x.squashfs"
	a.Metadata.Size = 8192

	f, err := files.OpenArtefactFile("testdata/UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7.snap")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := snap.NewSnapInspector()
	a.SetRequestPending(ins, "test")
	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)

	c.Check(a.Approved(), Equals, true)
	c.Check(a.Metadata.Type, Equals, "application/x.canonical.snap-package")
	c.Check(a.Metadata.Name, Equals, "word-salad")
	c.Check(a.Metadata.Vendor, Equals, "Alan Pope")
	c.Check(a.Metadata.Size, Equals, int64(8192))
	c.Check(a.Metadata.Version, Equals, "7")
	c.Check(a.Metadata.Architecture, Equals, "amd64")
	c.Check(a.Metadata.Description, Equals, "Word Salad - Password Generator")
	c.Check(a.ResponseInspection["snap"].Annotations, DeepEquals, Annotation{
		"snap-revision-assertion-header": map[string]string{
			"type":              "snap-revision",
			"authority-id":      "canonical",
			"developer-id":      "ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG",
			"sign-key-sha3-384": "BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul",
			"snap-id":           "UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7",
			"snap-revision":     "7",
			"snap-sha3-384":     "v0QSLRBEj2jMuEmtgYJrVjTFArf27nZBIqZrh87mZIF_ph_fmedOwOcZu4wpvLOs",
			"snap-size":         "8192",
			"timestamp":         "2019-02-27T17:30:26.742285Z",
		},
		"snap-declaration-assertion-header": map[string]string{
			"type":              "snap-declaration",
			"authority-id":      "canonical",
			"publisher-id":      "ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG",
			"series":            "16",
			"sign-key-sha3-384": "BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul",
			"snap-id":           "UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7",
			"snap-name":         "word-salad",
			"timestamp":         "2019-02-20T20:17:43.640421Z",
		},
		"account-assertion-header": map[string]string{
			"type":              "account",
			"account-id":        "ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG",
			"authority-id":      "canonical",
			"display-name":      "Alan Pope",
			"revision":          "2118",
			"sign-key-sha3-384": "BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul",
			"timestamp":         "2024-04-11T03:40:37.008746Z",
			"username":          "popey",
			"validation":        "starred",
		},
	})
}

func (s *snapSuite) TestSnapArtefactInspectorSkip(c *C) {
	a := metadata.NewArtefact()
	a.Metadata.Type = "application/octet-stream"
	a.Metadata.Size = 8192

	f, err := files.OpenArtefactFile("testdata/UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7.snap")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := snap.NewSnapInspector()
	a.SetRequestPending(ins, "test")
	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)

	c.Check(a.ResponseInspection["snap"], IsNil)
	c.Check(a.Rejected(), Equals, true)
}

func (s *snapSuite) TestSnapArtefactInspectorError(c *C) {
	for _, tc := range []struct {
		errorCase string
		errorMsg  string
	}{
		{"compute-digest", "cannot compute digest: compute digest error"},
		{"encode-digest", "cannot encode digest: encode digest error"},
		{"revision-assertion-download", "cannot retrieve snap-revision assertion: assertion download error"},
		{"declaration-assertion-download", "cannot retrieve snap-declaration assertion: assertion download error"},
		{"account-assertion-download", "cannot retrieve account assertion: assertion download error"},
	} {
		c.Logf("error case: %s", tc.errorCase)

		snap.MockComputeDigest(snap.ComputeDigestImpl)
		snap.MockEncodeDigest(snap.EncodeDigestImpl)
		snap.MockDownloadSnapRevisionAssertion(snap.DownloadSnapRevisionAssertionImpl)
		snap.MockDownloadSnapDeclarationAssertion(snap.DownloadSnapDeclarationAssertionImpl)
		snap.MockDownloadAccountAssertion(snap.DownloadAccountAssertionImpl)

		switch tc.errorCase {
		case "compute-digest":
			restorer := snap.MockComputeDigest(func(f io.Reader) ([]byte, error) {
				return nil, errors.New("compute digest error")
			})
			defer restorer()
		case "encode-digest":
			restorer := snap.MockEncodeDigest(func(b []byte) (string, error) {
				return "", errors.New("encode digest error")
			})
			defer restorer()
		case "revision-assertion-download":
			restorer := snap.MockDownloadSnapRevisionAssertion(func(s string) (*snap.Assertion, error) {
				return nil, errors.New("assertion download error")
			})
			defer restorer()
		case "declaration-assertion-download":
			restorer := snap.MockDownloadSnapDeclarationAssertion(func(s string) (*snap.Assertion, error) {
				return nil, errors.New("assertion download error")
			})
			defer restorer()
		case "account-assertion-download":
			restorer := snap.MockDownloadAccountAssertion(func(s string) (*snap.Assertion, error) {
				return nil, errors.New("assertion download error")
			})
			defer restorer()
		}

		a := metadata.NewArtefact()
		a.Metadata.Type = "application/x.squashfs"
		a.Metadata.Size = 8192

		f, err := files.OpenArtefactFile("testdata/UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7.snap")
		c.Assert(err, IsNil)
		defer f.Close()

		ins := snap.NewSnapInspector()
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtefact(f, a)
		c.Assert(err, Not(IsNil))
		c.Check(err.Error(), Equals, tc.errorMsg)
	}
}

func (s *snapSuite) TestSnapArtefactInspectorReject(c *C) {
	for _, tc := range []struct {
		rejectCase string
		reason     string
	}{
		{"snap-revision-signature-mismatch", "snap-revision assertion has invalid signature"},
		{"digest-mismatch", "snap-revision assertion digest mismatch"},
		{"snap-size-mismatch", "snap size mismatch in snap-revision assertion"},
		{"missing-snap-id", "cannot find snap ID in snap-revision assertion"},
		{"snap-declaration-signature-mismatch", "snap-declaration assertion has invalid signature"},
		{"missing-publisher-id", "cannot find publisher ID in snap-declaration assertion"},
		{"account-signature-mismatch", "account assertion has invalid signature"},
	} {
		c.Logf("rejection case: %s", tc.rejectCase)

		snap.MockComputeDigest(snap.ComputeDigestImpl)
		snap.MockEncodeDigest(snap.EncodeDigestImpl)
		snap.MockDownloadSnapRevisionAssertion(snap.DownloadSnapRevisionAssertionImpl)
		snap.MockDownloadSnapDeclarationAssertion(snap.DownloadSnapDeclarationAssertionImpl)
		snap.MockDownloadAccountAssertion(snap.DownloadAccountAssertionImpl)

		switch tc.rejectCase {
		case "snap-revision-signature-mismatch":
			restorer := snap.MockDownloadSnapRevisionAssertion(func(s string) (*snap.Assertion, error) {
				ast := &snap.Assertion{
					Signature: []byte("invalid-signature"),
					Header: map[string]string{
						"snap-size":     "8192",
						"snap-id":       "UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7",
						"snap-sha3-384": "v0QSLRBEj2jMuEmtgYJrVjTFArf27nZBIqZrh87mZIF_ph_fmedOwOcZu4wpvLOs",
					},
				}
				return ast, nil
			})
			defer restorer()
		case "digest-mismatch":
			restorer := snap.MockDownloadSnapRevisionAssertion(func(s string) (*snap.Assertion, error) {
				ast := &snap.Assertion{
					Header: map[string]string{
						"snap-size":     "8192",
						"snap-sha3-384": "invalid",
					},
				}
				return ast, nil
			})
			defer restorer()
		case "snap-size-mismatch":
			restorer := snap.MockDownloadSnapRevisionAssertion(func(s string) (*snap.Assertion, error) {
				ast := &snap.Assertion{
					Header: map[string]string{
						"snap-size": "123",
					},
				}
				return ast, nil
			})
			defer restorer()
		case "missing-snap-id":
			restorer := snap.MockDownloadSnapRevisionAssertion(func(s string) (*snap.Assertion, error) {
				ast := &snap.Assertion{
					Header: map[string]string{
						"snap-size":     "8192",
						"snap-sha3-384": "v0QSLRBEj2jMuEmtgYJrVjTFArf27nZBIqZrh87mZIF_ph_fmedOwOcZu4wpvLOs",
					},
				}
				return ast, nil
			})
			defer restorer()
		case "snap-declaration-signature-mismatch":
			restorer := snap.MockDownloadSnapDeclarationAssertion(func(s string) (*snap.Assertion, error) {
				ast := &snap.Assertion{
					Signature: []byte("invalid-signature"),
					Header: map[string]string{
						"publisher-id": "ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG",
					},
				}
				return ast, nil
			})
			defer restorer()
		case "missing-publisher-id":
			restorer := snap.MockDownloadSnapDeclarationAssertion(func(s string) (*snap.Assertion, error) {
				ast := &snap.Assertion{
					Header: map[string]string{},
				}
				return ast, nil
			})
			defer restorer()
		case "account-signature-mismatch":
			restorer := snap.MockDownloadAccountAssertion(func(s string) (*snap.Assertion, error) {
				ast := &snap.Assertion{
					Signature: []byte("invalid-signature"),
				}
				return ast, nil
			})
			defer restorer()
		}

		a := metadata.NewArtefact()
		a.Metadata.Type = "application/x.squashfs"
		a.Metadata.Size = 8192

		f, err := files.OpenArtefactFile("testdata/UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7.snap")
		c.Assert(err, IsNil)
		defer f.Close()

		ins := snap.NewSnapInspector()
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtefact(f, a)
		c.Assert(err, IsNil)

		c.Assert(a.Rejected(), Equals, true)
		c.Assert(a.ResponseInspection["snap"].Reason, Equals, tc.reason)
	}
}

func (s *snapSuite) TestSquashFsDetector(c *C) {
	for _, tc := range []struct {
		buffer []byte
		result bool
	}{
		{[]byte{}, false},
		{[]byte{0, 0, 0, 0}, false},
		{[]byte("hsq"), false},
		{[]byte("hsqx---"), false},
		{[]byte("hsqs---"), true},
	} {
		res := snap.SquashFsDetector(tc.buffer, uint32(len(tc.buffer)))
		c.Assert(res, Equals, tc.result)
	}
}
