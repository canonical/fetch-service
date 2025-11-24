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

package digests_test

import (
	"encoding/json"
	"fmt"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/metadata/digests"
)

const (
	MySha256 = "c1de7d7ad587318b4674ed029c7d22e33ce90268ca32c5b3dd1cff36511c7950"
)

func Test(t *testing.T) { TestingT(t) }

type digestsSuite struct{}

var _ = Suite(&digestsSuite{})

func (s *digestsSuite) TestSha1Digest(c *C) {
	h, err := digests.NewSha1Digest("290d07339dde2735121ab03e525ca6593c395a42")
	c.Assert(err, IsNil)
	c.Check(h.String(), Equals, "290d07339dde2735121ab03e525ca6593c395a42")
}

func (s *digestsSuite) TestSha1DigestError(c *C) {
	tc := []struct{ digest, msg string }{
		{"", "SHA1 digest length (0) is invalid"},                                               // empty string
		{"290d07339dde2735121ab03e525ca6593c395a", "SHA1 digest length (19) is invalid"},        // short string
		{"290d07339dde2735121ab03e525ca6593c395a4", "encoding/hex: odd length hex string"},      // odd string
		{"290d07339dde2735121ab03e525ca6593c395a4200", "SHA1 digest length (21) is invalid"},    // long string
		{"290d07339dde2735121ab03e525ca6593c395a42x", "encoding/hex: invalid byte: U+0078 'x'"}, // invalid character
	}

	for _, t := range tc {
		_, err := digests.NewSha1Digest(t.digest)
		c.Assert(err, Not(IsNil))
		c.Check(err.Error(), Equals, t.msg)
	}
}

func (s *digestsSuite) TestSha1DigestMarshal(c *C) {
	type Foo struct {
		Bar digests.Sha1Digest `json:"bar"`
	}

	h, err := digests.NewSha1Digest("290d07339dde2735121ab03e525ca6593c395a42")
	c.Assert(err, IsNil)
	j, err := json.Marshal(Foo{h})
	c.Assert(err, IsNil)
	c.Check(j, DeepEquals, []byte(`{"bar":"290d07339dde2735121ab03e525ca6593c395a42"}`))
}

func (s *digestsSuite) TestSha1DigestUnmarshal(c *C) {
	type Foo struct {
		Bar digests.Sha1Digest `json:"bar"`
	}

	for _, tc := range []struct {
		ydata  string
		errMsg string
		res    string
	}{
		{`"290d07339dde2735121ab03e525ca6593c395a42"`, "", "290d07339dde2735121ab03e525ca6593c395a42"},
		{`""`, "invalid SHA1 digest", ""},
		{`"abcd"`, "invalid SHA1 digest", ""},
		{`"an invalid digest string 01234567890abcd"`, "encoding/hex: invalid byte.*", ""},
		{"unquoted string", "invalid character .*", ""},
		{"{}", "invalid syntax", ""},
		{"false", "invalid syntax", ""},
	} {
		j := []byte(fmt.Sprintf(`{"bar": %s}`, tc.ydata))
		var foo Foo
		err := json.Unmarshal(j, &foo)
		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Check(foo.Bar.String(), Equals, tc.res)
		} else {
			c.Check(err, ErrorMatches, tc.errMsg)
		}
	}
}

func (s *digestsSuite) TestSha256Digest(c *C) {
	h, err := digests.NewSha256Digest("0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e")
	c.Assert(err, IsNil)
	c.Check(h.String(), Equals, "0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e")
}

func (s *digestsSuite) TestSha256DigestError(c *C) {
	tc := []struct{ digest, msg string }{
		{"", "SHA256 digest length (0) is invalid"},                                                                     // empty string
		{"0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb29", "SHA256 digest length (31) is invalid"},      // short string
		{"290d07339dde2735121ab03e525ca6593c395a4", "encoding/hex: odd length hex string"},                              // odd string
		{"0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e00", "SHA256 digest length (33) is invalid"},  // long string
		{"0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292ex", "encoding/hex: invalid byte: U+0078 'x'"}, // invalid character
	}

	for _, t := range tc {
		_, err := digests.NewSha256Digest(t.digest)
		c.Assert(err, Not(IsNil))
		c.Check(err.Error(), Equals, t.msg)
	}
}

func (s *digestsSuite) TestSha256DigestMarshal(c *C) {
	type Foo struct {
		Bar digests.Sha256Digest `json:"bar"`
	}

	h, err := digests.NewSha256Digest("0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e")
	c.Assert(err, IsNil)
	j, err := json.Marshal(Foo{h})
	c.Assert(err, IsNil)
	c.Check(j, DeepEquals, []byte(`{"bar":"0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e"}`))
}

func (s *digestsSuite) TestSha256DigestUnmarshal(c *C) {
	type Foo struct {
		Bar digests.Sha256Digest `json:"bar"`
	}
	for _, tc := range []struct {
		ydata  string
		errMsg string
		res    string
	}{
		{`"0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e"`, "", "0f9d4626df5afdf378004213b7f594cfb1ca0159ad00a4921fb40049dbcb292e"},
		{`""`, "invalid SHA256 digest", ""},
		{`"abcd"`, "invalid SHA256 digest", ""},
		{`"an invalid digest string 01234567890abcdef01234567890abcdef01234"`, "encoding/hex: invalid byte.*", ""},
		{"unquoted string", "invalid character .*", ""},
		{"{}", "invalid syntax", ""},
		{"false", "invalid syntax", ""},
	} {
		j := []byte(fmt.Sprintf(`{"bar": %s}`, tc.ydata))
		var foo Foo
		err := json.Unmarshal(j, &foo)
		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Check(foo.Bar.String(), Equals, tc.res)
		} else {
			c.Check(err, ErrorMatches, tc.errMsg)
		}
	}
}
