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
	"os"
	"testing"

	. "gopkg.in/check.v1"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/snap"
	"github.com/canonical/fetch-service/inspectors/snap/config"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type snapSuite struct{}

var _ = Suite(&snapSuite{})

func Test(t *testing.T) { TestingT(t) }

func getTestSnapInspectorConfig() config.SnapInspectorConfig {
	return config.SnapInspectorConfig{
		SnapDeclarationFilter: []config.AssertionFilter{},
	}
}

func fakeAccountAssertion(signKey string) (*snap.Assertion, error) {
	data, err := os.ReadFile("testdata/account.assert")
	if err != nil {
		return nil, err
	}
	return snap.NewAssertion(data)
}

func fakeSnapRevisionAssertion(snapSha3_384 string) (*snap.Assertion, error) {
	data, err := os.ReadFile("testdata/snap-revision.assert")
	if err != nil {
		return nil, err
	}
	return snap.NewAssertion(data)
}

func fakeSnapDeclarationAssertion(snapSha3_384 string) (*snap.Assertion, error) {
	data, err := os.ReadFile("testdata/snap-declaration.assert")
	if err != nil {
		return nil, err
	}
	return snap.NewAssertion(data)
}

func (s *snapSuite) Setup() {
}

func (s *snapSuite) TestSnapInspectorID(c *C) {
	ins := snap.NewSnapInspector(getTestSnapInspectorConfig())
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
		ins := snap.NewSnapInspector(getTestSnapInspectorConfig())
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
	ins := snap.NewSnapInspector(getTestSnapInspectorConfig())
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

	ins := snap.NewSnapInspector(getTestSnapInspectorConfig())
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

	ins := snap.NewSnapInspector(getTestSnapInspectorConfig())
	a.SetRequestPending(ins, "test")
	err = ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)

	c.Check(a.ResponseInspection["snap"], IsNil)
	c.Check(a.Rejected(), Equals, true)
}

func (s *snapSuite) TestSnapArtefactInspectorError(c *C) {
	restore := snap.MockDownloadAccountAssertion(fakeAccountAssertion)
	defer restore()

	restore = snap.MockDownloadSnapRevisionAssertion(fakeSnapRevisionAssertion)
	defer restore()

	restore = snap.MockDownloadSnapDeclarationAssertion(fakeSnapDeclarationAssertion)
	defer restore()

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

		snap.MockComputeDigest(snap.ComputeDigestImpl)
		snap.MockEncodeDigest(snap.EncodeDigestImpl)
		snap.MockDownloadSnapRevisionAssertion(fakeSnapRevisionAssertion)
		snap.MockDownloadSnapDeclarationAssertion(fakeSnapDeclarationAssertion)
		snap.MockDownloadAccountAssertion(fakeAccountAssertion)

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

		ins := snap.NewSnapInspector(getTestSnapInspectorConfig())
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtefact(f, a)
		c.Assert(err, Not(IsNil))
		c.Check(err.Error(), Equals, tc.errorMsg)
	}
}

func (s *snapSuite) TestSnapArtefactInspectorReject(c *C) {
	restore := snap.MockDownloadAccountAssertion(fakeAccountAssertion)
	defer restore()

	restore = snap.MockDownloadSnapRevisionAssertion(fakeSnapRevisionAssertion)
	defer restore()

	restore = snap.MockDownloadSnapDeclarationAssertion(fakeSnapDeclarationAssertion)
	defer restore()

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
		snap.MockDownloadSnapRevisionAssertion(fakeSnapRevisionAssertion)
		snap.MockDownloadSnapDeclarationAssertion(fakeSnapDeclarationAssertion)
		snap.MockDownloadAccountAssertion(fakeAccountAssertion)

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

		ins := snap.NewSnapInspector(getTestSnapInspectorConfig())
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

var declarationAssertion = `type: snap-declaration
format: 1
authority-id: canonical
revision: 9
series: 16
snap-id: KtwxgRlwCAVKFw92BUdt1WloH1Va3QPo
plugs:
  modem-manager:
    allow-auto-connection: true
publisher-id: canonical
slots:
  modem-manager:
    allow-connection: true
snap-name: modem-manager
timestamp: 2016-10-25T15:35:43.646671Z
sign-key-sha3-384: BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul

AcLBUgQAAQoABgUCWA970AAABF0QAMw+M28Rrm0m/3Gm5PYesQcQWKhGwmN0j3qfYG2LsSRiM0TU
j7K7hvCPc9v0P4sL6Ewv/CEZAkVxPYd9eUMqiyKYBRMp9QeiL7KW3RWdHok0FUN7ia7ZxcPlpKoM
uwV7qYDKktw/TJWX9bK15W6DnghlKtU464u7IqcHVmH2YzPBbcpJBuIhLHgYC2K5oj3ZvIjHqnV/
ELRDtwW3UTTkonycc2IUTCd10qu590z7DWzORWdts9ZARBJXfc3lohYkSd1v4wDYZHRO9RF/bJix
LBALp3kUR6X3OnLLJQAjVhIEY70B/5kLApuhrOpmi84Uawf+Uh91Ze++Bwatrw6QGw9cwkFgoLaj
9neiV4y6HvQh7gsgXap1XOZeOeWVMISgqaXGER78Lx6nc6/Loz8Yhjp4p9xi2Ia4j7fLpXMkWIU4
aoGudS1hQBsbeiNQvG6I+DraMN7xypMbOkGKwqNJ7prU63D3BmZiFl17ajT3SfffEO1/H6qqRVFS
A8X9HXVGPmI2TGst36cBgjdd9f+jj9ZqISKs8jdHfPKEpOBdH4wo1rodXO1y/GxZeP2Z710qep4t
8ynSRPi0l3boyM15D3IfnXMjLzUoace9vC6gltOHpW8GFPZvheQwknRvtfwRpZM2VsgaSw6cuz3+
7K/m9/Ff04A86/gvRlzduXIjEvKJ
`

func (s *snapSuite) TestCheckSnapDeclarationFilter(c *C) {
	for _, tc := range []struct {
		name   string
		value  []string
		errMsg string
	}{
		{"", []string{""}, ""},
		{"publisher-id", []string{"canonical"}, ""},
		{"publisher-id", []string{"foo", "canonical"}, ""},
		{"publisher-id", []string{""}, "attribute 'publisher-id' value 'canonical' is not allowed"},
		{"publisher-id", []string{"foo", "bar"}, "attribute 'publisher-id' value 'canonical' is not allowed"},
		{"publisher-id", []string{"foo"}, "attribute 'publisher-id' value 'canonical' is not allowed"},
		{"color", []string{"blue"}, "attribute 'color' not found in the snap-declaration assertion"},
	} {
		filter := []config.AssertionFilter{}
		if tc.name != "" {
			filter = []config.AssertionFilter{
				{Name: tc.name, Value: tc.value},
			}
		}

		cfg := config.SnapInspectorConfig{SnapDeclarationFilter: filter}

		a, err := snap.NewAssertion([]byte(declarationAssertion))
		c.Assert(err, IsNil)

		err = snap.CheckSnapDeclarationFilter(cfg, a)
		if tc.errMsg == "" {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err, ErrorMatches, tc.errMsg)
		}
	}
}

func (s *snapSuite) TestSnapDeclarationFilter(c *C) {
	restore := snap.MockDownloadAccountAssertion(fakeAccountAssertion)
	defer restore()

	restore = snap.MockDownloadSnapRevisionAssertion(fakeSnapRevisionAssertion)
	defer restore()

	restore = snap.MockDownloadSnapDeclarationAssertion(fakeSnapDeclarationAssertion)
	defer restore()

	for _, tc := range []struct {
		filter       []config.AssertionFilter
		rejectReason string
	}{

		{
			// no filters
			[]config.AssertionFilter{}, "",
		},
		{
			// filtering by matching publisher-id
			[]config.AssertionFilter{
				{Name: "publisher-id", Value: []string{"ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG"}},
			},
			"",
		},
		{
			// filtering by list including matching publisher-id
			[]config.AssertionFilter{
				{Name: "publisher-id", Value: []string{"canonical", "ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG"}},
			},
			"",
		},
		{
			// filtering by publisher-id and snap-name
			[]config.AssertionFilter{
				{Name: "publisher-id", Value: []string{"ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG"}},
				{Name: "snap-name", Value: []string{"word-salad"}},
			},
			"",
		},
		{
			// filtering by non-matching publisher-id
			[]config.AssertionFilter{
				{Name: "publisher-id", Value: []string{"canonical"}},
			},
			"failure on snap-declaration assertion attribute check",
		},
		{
			// filtering by matching publisher-id and non-maching snap-name
			[]config.AssertionFilter{
				{Name: "publisher-id", Value: []string{"ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG"}},
				{Name: "snap-name", Value: []string{"snorklemaster"}},
			},
			"failure on snap-declaration assertion attribute check",
		},
	} {
		cfg := config.SnapInspectorConfig{SnapDeclarationFilter: tc.filter}

		a := metadata.NewArtefact()
		a.Metadata.Type = "application/x.squashfs"
		a.Metadata.Size = 8192

		f, err := files.OpenArtefactFile("testdata/UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7.snap")
		c.Assert(err, IsNil)
		defer f.Close()

		ins := snap.NewSnapInspector(cfg)
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtefact(f, a)
		c.Assert(err, IsNil)

		if tc.rejectReason == "" {
			c.Assert(a.ResponseInspection["snap"].Opinion, Equals, opinions.Approved)
		} else {
			c.Assert(a.ResponseInspection["snap"].Opinion, Equals, opinions.Rejected)
			c.Assert(a.ResponseInspection["snap"].Reason, Equals, tc.rejectReason)
		}
	}
}
