// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2025 Canonical Ltd.
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
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type snapSuite struct {
	sl logger.Logger
}

var _ = Suite(&snapSuite{logger.NewSessionLogger("test")})

func Test(t *testing.T) { TestingT(t) }

func getTestSnapInspectorConfig() config.SnapInspectorConfig {
	return config.SnapInspectorConfig{
		SnapDeclarationFilter: []config.AssertionFilter{},
	}
}

func fakeAccountAssertion(signKey string, sl logger.Logger) (*snap.Assertion, error) {
	data, err := os.ReadFile("testdata/account.assert")
	if err != nil {
		return nil, err
	}
	return snap.NewAssertion(data)
}

func fakeSnapRevisionAssertion(snapSha3_384 string, sl logger.Logger) (*snap.Assertion, error) {
	data, err := os.ReadFile("testdata/snap-revision.assert")
	if err != nil {
		return nil, err
	}
	return snap.NewAssertion(data)
}

func fakeSnapDeclarationAssertion(snapSha3_384 string, sl logger.Logger) (*snap.Assertion, error) {
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

type snapInspectRequestTest struct {
	url      string // The artifact request URL
	approved bool   // Whether the request should be approved
}

var snapInspectRequestTests = []snapInspectRequestTest{{
	url:      "https://api.snapcraft.io:443/api/v1/snaps/download/foo_42.snap",
	approved: true,
}, {
	url:      "https://x.snapcraftcontent.com:443/subdir/foo_42.snap?",
	approved: true,
}, {
	url:      "https://api.snapcraft.io:443/v2/snaps/download/foo_42.snap",
	approved: false,
}, {
	url:      "https://x.snapcraftcontent.com:443/subdir/foo_42.snap",
	approved: false,
}, {
	url:      "https://api.snapcraft.io:443/v3/snaps/download/foo_42.snap",
	approved: false,
}, {
	url:      "http://api.snapcraft.io/v2/snaps/download/foo_42.snap",
	approved: false,
}, {
	url:      "https://x.snapcraftcontent.com:443/subdir/foo_42.snap",
	approved: false,
}, {
	url:      "https://api.snapcraft.io/v2/snaps/info",
	approved: false,
}}

func (s *snapSuite) TestSnapInspectRequest(c *C) {
	for _, tc := range snapInspectRequestTests {
		ins := snap.NewSnapInspector(getTestSnapInspectorConfig())
		a := metadata.NewArtifact()
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

func (s *snapSuite) TestSnapInspectRequestError(c *C) {
	ins := snap.NewSnapInspector(getTestSnapInspectorConfig())
	a := metadata.NewArtifact()
	a.CurrentDownload = metadata.Download{URL: "::"}

	err := ins.InspectRequest(a)
	c.Assert(err, ErrorMatches, ".*: missing protocol scheme")
}

func (s *snapSuite) TestSnapArtifactInspector(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/x.squashfs"
	a.Metadata.Size = 8192

	f, err := files.OpenArtifactFile("testdata/UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7.snap")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := snap.NewSnapInspector(getTestSnapInspectorConfig())
	a.SetRequestPending(ins, "test")
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	c.Check(a.Approved(), Equals, true)
	c.Check(a.Metadata.Type, Equals, "application/x.canonical.snap-package")
	c.Check(a.Metadata.Name, Equals, "word-salad")
	c.Check(a.Metadata.Vendor, Equals, "Alan Pope")
	c.Check(a.Metadata.Size, Equals, int64(8192))
	c.Check(a.Metadata.Version, Equals, "0.1")
	c.Check(a.Metadata.StoreRevision, Equals, "7")
	c.Check(a.Metadata.Architecture, Equals, "amd64")
	c.Check(a.Metadata.Description, Equals, "Word Salad - Password Generator")
	c.Check(a.Metadata.ContentID, Equals, "UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7")
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

func (s *snapSuite) TestSnapArtifactInspectorSkip(c *C) {
	a := metadata.NewArtifact()
	a.Metadata.Type = "application/octet-stream"
	a.Metadata.Size = 8192

	f, err := files.OpenArtifactFile("testdata/UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7.snap")
	c.Assert(err, IsNil)
	defer f.Close()

	ins := snap.NewSnapInspector(getTestSnapInspectorConfig())
	a.SetRequestPending(ins, "test")
	err = ins.InspectArtifact(f, a)
	c.Assert(err, IsNil)

	c.Check(a.ResponseInspection["snap"], IsNil)
	c.Check(a.Rejected(), Equals, true)
}

type snapArtifactInspectorErrorTest struct {
	errorCase string // A string identifying this error case
	errorMsg  string // The expected error message
}

var snapArtifactInspectorErrorTests = []snapArtifactInspectorErrorTest{{
	errorCase: "compute-digest",
	errorMsg:  "cannot compute digest: compute digest error",
}, {
	errorCase: "encode-digest",
	errorMsg:  "cannot encode digest: encode digest error",
}, {
	errorCase: "revision-assertion-download",
	errorMsg:  "",
}, {
	errorCase: "declaration-assertion-download",
	errorMsg:  "cannot retrieve snap-declaration assertion: assertion download error",
}, {
	errorCase: "account-assertion-download",
	errorMsg:  "cannot retrieve account assertion: assertion download error",
}}

func (s *snapSuite) TestSnapArtifactInspectorError(c *C) {
	restore := snap.MockDownloadAccountAssertion(fakeAccountAssertion)
	defer restore()

	restore = snap.MockDownloadSnapRevisionAssertion(fakeSnapRevisionAssertion)
	defer restore()

	restore = snap.MockDownloadSnapDeclarationAssertion(fakeSnapDeclarationAssertion)
	defer restore()

	for _, tc := range snapArtifactInspectorErrorTests {
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
			restorer := snap.MockDownloadSnapRevisionAssertion(func(s string, sl logger.Logger) (*snap.Assertion, error) {
				return nil, errors.New("assertion download error")
			})
			defer restorer()
		case "declaration-assertion-download":
			restorer := snap.MockDownloadSnapDeclarationAssertion(func(s string, sl logger.Logger) (*snap.Assertion, error) {
				return nil, errors.New("assertion download error")
			})
			defer restorer()
		case "account-assertion-download":
			restorer := snap.MockDownloadAccountAssertion(func(s string, sl logger.Logger) (*snap.Assertion, error) {
				return nil, errors.New("assertion download error")
			})
			defer restorer()
		}

		a := metadata.NewArtifact()
		a.Metadata.Type = "application/x.squashfs"
		a.Metadata.Size = 8192

		f, err := files.OpenArtifactFile("testdata/UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7.snap")
		c.Assert(err, IsNil)
		defer f.Close()

		ins := snap.NewSnapInspector(getTestSnapInspectorConfig())
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtifact(f, a)
		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(a.ResponseInspection, HasLen, 0)
		} else {
			c.Assert(err, Not(IsNil))
			c.Check(err.Error(), Equals, tc.errorMsg)
		}
	}
}

type snapArtifactInspectorRejectTest struct {
	rejectCase string // A string identifying this rejection case
	reason     string // The expected reason why the artifact was rejected
}

var snapArtifactInspectorRejectTests = []snapArtifactInspectorRejectTest{{
	rejectCase: "snap-revision-signature-mismatch",
	reason:     "snap-revision assertion has invalid signature",
}, {
	rejectCase: "digest-mismatch",
	reason:     "snap-revision assertion digest mismatch",
}, {
	rejectCase: "snap-size-mismatch",
	reason:     "snap size mismatch in snap-revision assertion",
}, {
	rejectCase: "missing-snap-id",
	reason:     "cannot find snap ID in snap-revision assertion",
}, {
	rejectCase: "snap-declaration-signature-mismatch",
	reason:     "snap-declaration assertion has invalid signature",
}, {
	rejectCase: "missing-publisher-id",
	reason:     "cannot find publisher ID in snap-declaration assertion",
}, {
	rejectCase: "account-signature-mismatch",
	reason:     "account assertion has invalid signature",
}}

func (s *snapSuite) TestSnapArtifactInspectorReject(c *C) {
	restore := snap.MockDownloadAccountAssertion(fakeAccountAssertion)
	defer restore()

	restore = snap.MockDownloadSnapRevisionAssertion(fakeSnapRevisionAssertion)
	defer restore()

	restore = snap.MockDownloadSnapDeclarationAssertion(fakeSnapDeclarationAssertion)
	defer restore()

	for _, tc := range snapArtifactInspectorRejectTests {
		c.Logf("rejection case: %s", tc.rejectCase)

		snap.MockComputeDigest(snap.ComputeDigestImpl)
		snap.MockEncodeDigest(snap.EncodeDigestImpl)
		snap.MockDownloadSnapRevisionAssertion(fakeSnapRevisionAssertion)
		snap.MockDownloadSnapDeclarationAssertion(fakeSnapDeclarationAssertion)
		snap.MockDownloadAccountAssertion(fakeAccountAssertion)

		switch tc.rejectCase {
		case "snap-revision-signature-mismatch":
			restorer := snap.MockDownloadSnapRevisionAssertion(func(s string, _ logger.Logger) (*snap.Assertion, error) {
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
			restorer := snap.MockDownloadSnapRevisionAssertion(func(s string, _ logger.Logger) (*snap.Assertion, error) {
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
			restorer := snap.MockDownloadSnapRevisionAssertion(func(s string, _ logger.Logger) (*snap.Assertion, error) {
				ast := &snap.Assertion{
					Header: map[string]string{
						"snap-size": "123",
					},
				}
				return ast, nil
			})
			defer restorer()
		case "missing-snap-id":
			restorer := snap.MockDownloadSnapRevisionAssertion(func(s string, _ logger.Logger) (*snap.Assertion, error) {
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
			restorer := snap.MockDownloadSnapDeclarationAssertion(func(s string, _ logger.Logger) (*snap.Assertion, error) {
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
			restorer := snap.MockDownloadSnapDeclarationAssertion(func(s string, _ logger.Logger) (*snap.Assertion, error) {
				ast := &snap.Assertion{
					Header: map[string]string{},
				}
				return ast, nil
			})
			defer restorer()
		case "account-signature-mismatch":
			restorer := snap.MockDownloadAccountAssertion(func(s string, _ logger.Logger) (*snap.Assertion, error) {
				ast := &snap.Assertion{
					Signature: []byte("invalid-signature"),
				}
				return ast, nil
			})
			defer restorer()
		}

		a := metadata.NewArtifact()
		a.Metadata.Type = "application/x.squashfs"
		a.Metadata.Size = 8192

		f, err := files.OpenArtifactFile("testdata/UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7.snap")
		c.Assert(err, IsNil)
		defer f.Close()

		ins := snap.NewSnapInspector(getTestSnapInspectorConfig())
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		c.Assert(a.Rejected(), Equals, true)
		c.Assert(a.ResponseInspection["snap"].Reason, Equals, tc.reason)
	}
}

type squashFsDetectorTest struct {
	buffer   []byte // The contents of the format detector buffer
	detected bool   // Whether this should be detected as SquashFS
}

var squashFsDetectorTests = []squashFsDetectorTest{{
	buffer:   []byte{},
	detected: false,
}, {
	buffer:   []byte{0, 0, 0, 0},
	detected: false,
}, {
	buffer:   []byte("hsq"),
	detected: false,
}, {
	buffer:   []byte("hsqx---"),
	detected: false,
}, {
	buffer:   []byte("hsqs---"),
	detected: true,
}}

func (s *snapSuite) TestSquashFsDetector(c *C) {
	for _, tc := range squashFsDetectorTests {
		res := snap.SquashFsDetector(tc.buffer, uint32(len(tc.buffer)))
		c.Assert(res, Equals, tc.detected)
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

type checkSnapDeclarationFilterTest struct {
	name     string   // The snap-declaration assertion field name
	value    []string // The snap-declaration assertion field value
	errorMsg string   // The expected error message, or empty
}

var checkSnapDeclarationFilterTests = []checkSnapDeclarationFilterTest{{
	name:     "",
	value:    []string{""},
	errorMsg: "",
}, {
	name:     "publisher-id",
	value:    []string{"canonical"},
	errorMsg: "",
}, {
	name:     "publisher-id",
	value:    []string{"foo", "canonical"},
	errorMsg: "",
}, {
	name:     "publisher-id",
	value:    []string{""},
	errorMsg: "attribute 'publisher-id' value 'canonical' is not allowed",
}, {
	name:     "publisher-id",
	value:    []string{"foo", "bar"},
	errorMsg: "attribute 'publisher-id' value 'canonical' is not allowed",
}, {
	name:     "publisher-id",
	value:    []string{"foo"},
	errorMsg: "attribute 'publisher-id' value 'canonical' is not allowed",
}, {
	name:     "color",
	value:    []string{"blue"},
	errorMsg: "attribute 'color' not found in the snap-declaration assertion",
}}

func (s *snapSuite) TestCheckSnapDeclarationFilter(c *C) {
	for _, tc := range checkSnapDeclarationFilterTests {
		filter := []config.AssertionFilter{}
		if tc.name != "" {
			filter = []config.AssertionFilter{
				{Name: tc.name, Value: tc.value},
			}
		}

		cfg := config.SnapInspectorConfig{SnapDeclarationFilter: filter}

		a, err := snap.NewAssertion([]byte(declarationAssertion))
		c.Assert(err, IsNil)

		err = snap.CheckSnapDeclarationFilter(cfg, a, s.sl)
		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
		} else {
			c.Assert(err, ErrorMatches, tc.errorMsg)
		}
	}
}

type snapDeclarationFilterTest struct {
	filter       []config.AssertionFilter // The declaration filter to apply
	rejectReason string                   // The filter rejection reason, if any
}

var snapDeclarationFilterTests = []snapDeclarationFilterTest{{
	// no filters
	filter:       []config.AssertionFilter{},
	rejectReason: "",
}, {
	// filtering by matching publisher-id
	filter: []config.AssertionFilter{
		{Name: "publisher-id", Value: []string{"ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG"}},
	},
	rejectReason: "",
}, {
	// filtering by list including matching publisher-id
	filter: []config.AssertionFilter{
		{Name: "publisher-id", Value: []string{"canonical", "ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG"}},
	},
	rejectReason: "",
}, {
	// filtering by publisher-id and snap-name
	filter: []config.AssertionFilter{
		{Name: "publisher-id", Value: []string{"ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG"}},
		{Name: "snap-name", Value: []string{"word-salad"}},
	},
	rejectReason: "",
}, {
	// filtering by non-matching publisher-id
	filter: []config.AssertionFilter{
		{Name: "publisher-id", Value: []string{"canonical"}},
	},
	rejectReason: "failure on snap-declaration assertion attribute check",
}, {
	// filtering by matching publisher-id and non-maching snap-name
	filter: []config.AssertionFilter{
		{Name: "publisher-id", Value: []string{"ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG"}},
		{Name: "snap-name", Value: []string{"snorklemaster"}},
	},
	rejectReason: "failure on snap-declaration assertion attribute check",
}}

func (s *snapSuite) TestSnapDeclarationFilter(c *C) {
	restore := snap.MockDownloadAccountAssertion(fakeAccountAssertion)
	defer restore()

	restore = snap.MockDownloadSnapRevisionAssertion(fakeSnapRevisionAssertion)
	defer restore()

	restore = snap.MockDownloadSnapDeclarationAssertion(fakeSnapDeclarationAssertion)
	defer restore()

	for _, tc := range snapDeclarationFilterTests {
		cfg := config.SnapInspectorConfig{SnapDeclarationFilter: tc.filter}

		a := metadata.NewArtifact()
		a.Metadata.Type = "application/x.squashfs"
		a.Metadata.Size = 8192

		f, err := files.OpenArtifactFile("testdata/UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7.snap")
		c.Assert(err, IsNil)
		defer f.Close()

		ins := snap.NewSnapInspector(cfg)
		a.SetRequestPending(ins, "test")
		err = ins.InspectArtifact(f, a)
		c.Assert(err, IsNil)

		if tc.rejectReason == "" {
			c.Assert(a.ResponseInspection["snap"].Opinion, Equals, opinions.Approved)
		} else {
			c.Assert(a.ResponseInspection["snap"].Opinion, Equals, opinions.Rejected)
			c.Assert(a.ResponseInspection["snap"].Reason, Equals, tc.rejectReason)
		}
	}
}
