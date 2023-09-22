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
	"encoding/json"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
)

const (
	MySha256 = "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950"
)

func Test(t *testing.T) { TestingT(t) }

type metadataSuite struct{}

func (t *metadataSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&metadataSuite{})

func (s *metadataSuite) TestSha1Digest(c *C) {
	h, err := metadata.NewSha1Digest("290d07339dde2735121ab03e525ca6593c395a42")
	c.Assert(err, IsNil)
	c.Check(h.String(), Equals, "290d07339dde2735121ab03e525ca6593c395a42")
}

func (s *metadataSuite) TestSha1DigestError(c *C) {
	tc := []struct{ digest, msg string }{
		{"", "SHA1 digest length (0) is invalid"},                                               // empty string
		{"290d07339dde2735121ab03e525ca6593c395a", "SHA1 digest length (19) is invalid"},        // short string
		{"290d07339dde2735121ab03e525ca6593c395a4", "encoding/hex: odd length hex string"},      // odd string
		{"290d07339dde2735121ab03e525ca6593c395a4200", "SHA1 digest length (21) is invalid"},    // long string
		{"290d07339dde2735121ab03e525ca6593c395a42x", "encoding/hex: invalid byte: U+0078 'x'"}, // invalid character
	}

	for _, t := range tc {
		_, err := metadata.NewSha1Digest(t.digest)
		c.Assert(err, Not(IsNil))
		c.Check(err.Error(), Equals, t.msg)
	}
}

func (s *metadataSuite) TestSha1DigestMarshal(c *C) {
	type Foo struct {
		Bar metadata.Sha1Digest `json:"bar"`
	}

	h, _ := metadata.NewSha1Digest("290d07339dde2735121ab03e525ca6593c395a42")
	j, err := json.Marshal(Foo{h})
	c.Assert(err, IsNil)
	c.Check(j, DeepEquals, []byte(`{"bar":"290d07339dde2735121ab03e525ca6593c395a42"}`))
}

func (s *metadataSuite) TestSha1DigestUnmarshal(c *C) {
	j := []byte(`{"bar":"290d07339dde2735121ab03e525ca6593c395a42"}`)

	type Foo struct {
		Bar metadata.Sha1Digest `json:"bar"`
	}

	var foo Foo
	err := json.Unmarshal(j, &foo)
	c.Assert(err, IsNil)
	c.Check(foo.Bar.String(), Equals, "290d07339dde2735121ab03e525ca6593c395a42")
}

func (s *metadataSuite) TestSha256Digest(c *C) {
	h, err := metadata.NewSha256Digest("0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e")
	c.Assert(err, IsNil)
	c.Check(h.String(), Equals, "0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e")
}

func (s *metadataSuite) TestSha256DigestError(c *C) {
	tc := []struct{ digest, msg string }{
		{"", "SHA256 digest length (0) is invalid"},                                                                     // empty string
		{"0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb29", "SHA256 digest length (31) is invalid"},      // short string
		{"290d07339dde2735121ab03e525ca6593c395a4", "encoding/hex: odd length hex string"},                              // odd string
		{"0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e00", "SHA256 digest length (33) is invalid"},  // long string
		{"0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292ex", "encoding/hex: invalid byte: U+0078 'x'"}, // invalid character
	}

	for _, t := range tc {
		_, err := metadata.NewSha256Digest(t.digest)
		c.Assert(err, Not(IsNil))
		c.Check(err.Error(), Equals, t.msg)
	}
}
func (s *metadataSuite) TestSha256DigestMarshal(c *C) {
	type Foo struct {
		Bar metadata.Sha256Digest `json:"bar"`
	}

	h, _ := metadata.NewSha256Digest("0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e")
	j, err := json.Marshal(Foo{h})
	c.Assert(err, IsNil)
	c.Check(j, DeepEquals, []byte(`{"bar":"0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e"}`))
}

func (s *metadataSuite) TestSha256DigestUnmarshal(c *C) {
	j := []byte(`{"bar":"0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e"}`)

	type Foo struct {
		Bar metadata.Sha256Digest `json:"bar"`
	}

	var foo Foo
	err := json.Unmarshal(j, &foo)
	c.Assert(err, IsNil)
	c.Check(foo.Bar.String(), Equals, "0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e")
}

/*
func (s *metadataSuite) TestAnnotation(c *C) {
	md := metadata.Metadata{}

	md.Annotate("test.foo", metadata.AnnotationValue{"text": "test"})
	md.Annotate("test.bar", metadata.AnnotationValue{})

	c.Assert(md.Annotations, HasLen, 2)
	c.Assert(md.Annotations["test.foo"].Value, DeepEquals, metadata.AnnotationValue{"text": "test"})
	c.Assert(md.Annotations["test.bar"].Value, HasLen, 0)
}
*/

/*
func (s *metadataSuite) TestContextReleasePackages(c *C) {
	ctx := metadata.NewInspectionContext()
	c.Assert(ctx, Not(IsNil))

	p := metadata.AptReleasePackages{
		Path:   "path/to/Packages.xz",
		Size:   12345,
		Vendor: "Acme",
	}

	releaseDigest, _ := metadata.NewSha256Digest(MySha256)
	packagesDigest, _ := metadata.NewSha256Digest("f1d6e0e435c851796ddc982230070bf5f6c313fade049f31e2983e5b26c43a72")
	otherDigest, _ := metadata.NewSha256Digest("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	ctx.AddReleasePackages(releaseDigest, packagesDigest, p)

	digest, _, ok := ctx.GetReleasePackages(otherDigest)
	c.Assert(ok, Equals, false)
	c.Assert(digest, Equals, metadata.Sha256Digest{})

	digest, q, ok := ctx.GetReleasePackages(packagesDigest)
	c.Assert(ok, Equals, true)
	c.Assert(digest, Equals, releaseDigest)
	c.Assert(q, DeepEquals, p)
}

func (s *metadataSuite) TestContextPackagesEntry(c *C) {
	ctx := metadata.NewInspectionContext()
	c.Assert(ctx, Not(IsNil))

	e := metadata.AptPackagesEntry{
		Package:      "hello",
		Version:      "1.2.3",
		Architecture: "amd64",
		Size:         1337,
	}

	packagesDigest, _ := metadata.NewSha256Digest(MySha256)
	helloDigest, _ := metadata.NewSha256Digest("e24f8496e591bfa9fc493ab6bbb702b8ee60a47d974139c17f20f095dd0d5670")
	otherDigest, _ := metadata.NewSha256Digest("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	ctx.AddPackagesEntry(packagesDigest, helloDigest, e)

	digest, _, ok := ctx.GetPackagesEntry(otherDigest)
	c.Assert(ok, Equals, false)
	c.Assert(digest, Equals, metadata.Sha256Digest{})

	digest, f, ok := ctx.GetPackagesEntry(helloDigest)
	c.Assert(ok, Equals, true)
	c.Assert(digest, Equals, packagesDigest)
	c.Assert(f, DeepEquals, e)
}
*/
