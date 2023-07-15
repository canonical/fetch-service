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
	"os"
	"path/filepath"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/metadata"
)

func Test(t *testing.T) { TestingT(t) }

type metadataSuite struct{}

var _ = Suite(&metadataSuite{})

func (s *metadataSuite) TestSha1Digest(c *C) {
	h, err := metadata.NewSha1Digest("290d07339dde2735121ab03e525ca6593c395a42")
	c.Assert(err, IsNil)
	c.Check(h.String(), Equals, "290d07339dde2735121ab03e525ca6593c395a42")
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

func (s *metadataSuite) TestAnnotation(c *C) {
	md := metadata.Metadata{}

	md.Annotate(metadata.Notice, "test.notice", "annotation text")

	data := metadata.AnnotationDetails{"text": "test"}
	md.Annotate(metadata.PolicyViolation, "test.violation", "more annotation text").SetDetails(data)

	c.Assert(md.Annotations, HasLen, 2)
	c.Assert(md.Annotations["test.notice"].Kind, Equals, metadata.Notice)
	c.Assert(md.Annotations["test.notice"].Origin, Equals, "unknown")
	c.Assert(md.Annotations["test.notice"].Value, Equals, "annotation text")
	c.Assert(md.Annotations["test.notice"].Details, HasLen, 0)
	c.Assert(md.Annotations["test.violation"].Kind, Equals, metadata.PolicyViolation)
	c.Assert(md.Annotations["test.violation"].Origin, Equals, "unknown")
	c.Assert(md.Annotations["test.violation"].Value, Equals, "more annotation text")
	c.Assert(md.Annotations["test.violation"].Details, DeepEquals, metadata.AnnotationDetails{"text": "test"})
}

func (s *metadataSuite) TestRunInspectors(c *C) {
	ctx := metadata.NewInspectionContext()

	dir := c.MkDir()
	data := []byte("Measure twice, saw once.\n")
	err := os.WriteFile(filepath.Join(dir, "290d07339dde2735121ab03e525ca6593c395a42.bin"), data, 0644)
	c.Assert(err, IsNil)

	h, _ := metadata.NewSha1Digest("290d07339dde2735121ab03e525ca6593c395a42")
	md := &metadata.Metadata{Sha1: h}
	di := &metadata.DownloadInfo{ContentType: "text/plain", Sha1: h}

	err = ctx.RunInspectors(dir, md, di)
	c.Assert(err, IsNil)
	c.Assert(md.Type, Equals, "text/plain; charset=utf-8")

	// TODO: improve this test to see if registered inspectors ran as expected
}

func (s *metadataSuite) TestDefaultInspector(c *C) {
	md := &metadata.Metadata{Type: "application/unit-test"}
	di := &metadata.DownloadInfo{}

	var iface metadata.Inspector
	ins := metadata.DefaultInspector{}
	c.Assert(ins, Implements, &iface)

	stop, err := ins.Inspect("any-filename", md, di, nil)
	c.Assert(err, IsNil)
	c.Assert(stop, Equals, true)
	c.Assert(md.Annotations, HasLen, 1)
	c.Assert(md.Annotations["default.unknown"].Kind, Equals, metadata.Warning)
	c.Assert(md.Annotations["default.unknown"].Origin, Equals, "metadata.defaultInspector")
	c.Assert(md.Annotations["default.unknown"].Value, Equals, "unknown file format")
	c.Assert(md.Annotations["default.unknown"].Details, HasLen, 0)
}

func (s *metadataSuite) TestContextReleasePackages(c *C) {
	ctx := metadata.NewInspectionContext()
	c.Assert(ctx, Not(IsNil))

	p := metadata.AptReleasePackages{
		Path:   "path/to/Packages.xz",
		Size:   12345,
		Vendor: "Acme",
	}

	releaseDigest, _ := metadata.NewSha1Digest("992b22a7457f7f75b4cfa197393993ebdaa64faf")
	packagesDigest, _ := metadata.NewSha256Digest("f1d6e0e435c851796ddc982230070bf5f6c313fade049f31e2983e5b26c43a72")
	otherDigest, _ := metadata.NewSha256Digest("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	ctx.AddReleasePackages(releaseDigest, packagesDigest, p)

	digest, _, ok := ctx.GetReleasePackages(otherDigest)
	c.Assert(ok, Equals, false)
	c.Assert(digest, Equals, metadata.Sha1Digest{})

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

	packagesDigest, _ := metadata.NewSha1Digest("a47171134c87396f681a59920c68b3cffdf52851")
	helloDigest, _ := metadata.NewSha256Digest("e24f8496e591bfa9fc493ab6bbb702b8ee60a47d974139c17f20f095dd0d5670")
	otherDigest, _ := metadata.NewSha256Digest("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	ctx.AddPackagesEntry(packagesDigest, helloDigest, e)

	digest, _, ok := ctx.GetPackagesEntry(otherDigest)
	c.Assert(ok, Equals, false)
	c.Assert(digest, Equals, metadata.Sha1Digest{})

	digest, f, ok := ctx.GetPackagesEntry(helloDigest)
	c.Assert(ok, Equals, true)
	c.Assert(digest, Equals, packagesDigest)
	c.Assert(f, DeepEquals, e)
}
