// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024 Canonical Ltd.
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

package config_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/service/config"
)

func Test(t *testing.T) { TestingT(t) }

type configSuite struct{}

func (t *configSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&configSuite{})

func (t *configSuite) TestMarshalACLPolicy(c *C) {
	type FooStruct struct {
		Foo config.ACLPolicy `yaml:"foo"`
	}

	for _, tc := range []struct {
		val    config.ACLPolicy
		errMsg string
		s      string
	}{
		{config.Allow, "", "foo: allow\n"},
		{config.Deny, "", "foo: deny\n"},
		{42, "invalid ACL policy", ""},
	} {
		d, err := yaml.Marshal(FooStruct{Foo: tc.val})
		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(string(d), Equals, tc.s)
		} else {
			c.Assert(err.Error(), Equals, tc.errMsg)
		}
	}
}

func (t *configSuite) TestUnmarshalACLPolicy(c *C) {
	type FooStruct struct {
		Foo config.ACLPolicy `yaml:"foo"`
	}

	for _, tc := range []struct {
		s      string
		errMsg string
		val    config.ACLPolicy
	}{
		{"foo: allow", "", config.Allow},
		{"foo: deny", "", config.Deny},
		{"foo: invalid", "invalid ACL policy", 0},
	} {
		var foo FooStruct
		err := yaml.Unmarshal([]byte(tc.s), &foo)
		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(foo.Foo, Equals, tc.val)
		} else {
			c.Assert(err.Error(), Equals, tc.errMsg)
		}
	}
}

func (t *configSuite) TestACLPolicyString(c *C) {
	c.Assert(config.Allow.String(), Equals, "allow")
	c.Assert(config.Deny.String(), Equals, "deny")
}

func (t *configSuite) TestMarshalIPNet(c *C) {
	type FooStruct struct {
		Foo config.IPNet `yaml:"foo"`
	}

	for _, tc := range []struct {
		val    string
		errMsg string
		s      string
	}{
		{"10.42.42.0/24", "", "foo: 10.42.42.0/24\n"},
		{"10.42.42.10/24", "", "foo: 10.42.42.0/24\n"},
		{"::1/128", "", "foo: ::1/128\n"},
		{"0:0:0:0::1/128", "", "foo: ::1/128\n"},
		{"ff80::1/128", "", "foo: ff80::1/128\n"},
	} {
		_, ipnet, err := net.ParseCIDR(tc.val)
		c.Assert(err, IsNil)

		d, err := yaml.Marshal(FooStruct{Foo: config.IPNet{*ipnet}})
		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Check(string(d), Equals, tc.s)
		} else {
			c.Assert(err.Error(), Equals, tc.errMsg)
		}
	}
}

func (t *configSuite) TestUnmarshalIPNet(c *C) {
	type FooStruct struct {
		Foo config.IPNet `yaml:"foo"`
	}

	for _, tc := range []struct {
		s      string
		errMsg string
		val    string
	}{
		{"foo: 10.42.42.0/24", "", "10.42.42.0/24"},
		{"foo: 10.42.42.10", "", "10.42.42.10/32"},
		{"foo: ::1/128", "", "::1/128"},
		{"foo: 0:0:0:0::1/128", "", "::1/128"},
		{"foo: ff80::1/64", "", "ff80::/64"},
		{"foo: FF80::1/64", "", "ff80::/64"},
		{"foo: xxx", "invalid CIDR address: xxx", ""},
	} {
		var foo FooStruct
		err := yaml.Unmarshal([]byte(tc.s), &foo)
		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Check(foo.Foo.String(), Equals, tc.val)
		} else {
			c.Assert(err.Error(), Equals, tc.errMsg)
		}
	}
}

var proxyConfig = `
http-proxy:
  policy: deny
  rules:
    - dst: [
        1.2.0.0/16,
	ffd0::/16,
      ]
      access: allow
    - dst: [
        1.2.3.4, 1.2.3.5,
        1.2.4.0/24,
      ]
      access: deny
    - dst: [
        1.2.4.3
      ]
      access: allow
`

func (t *configSuite) TestGetSetHttpProxyConfig(c *C) {
	dir := c.MkDir()
	cfgFile := filepath.Join(dir, "acl.yaml")
	err := os.WriteFile(cfgFile, []byte(proxyConfig), 0644)
	c.Assert(err, IsNil)

	// Load rules from file
	err = config.LoadHttpProxyRules(dir)
	c.Assert(err, IsNil)

	cfg := config.GetHttpProxyConfig()
	c.Check(cfg.Policy, Equals, config.Deny)
	c.Check(cfg.Rules, DeepEquals, []config.Rule{
		config.Rule{
			Access: config.Allow,
			Dst: []config.IPNet{
				ipNet("1.2.0.0/16"),
				ipNet("ffd0::/16"),
			},
		},
		config.Rule{
			Access: config.Deny,
			Dst: []config.IPNet{
				ipNet("1.2.3.4/32"),
				ipNet("1.2.3.5/32"),
				ipNet("1.2.4.0/24"),
			},
		},
		config.Rule{
			Access: config.Allow,
			Dst: []config.IPNet{
				ipNet("1.2.4.3/32"),
			},
		},
	})

	// Verify that loaded rules are a copy
	cfg.Policy = config.Allow
	cfg.Rules[1].Dst = []config.IPNet{}

	cfg2 := config.GetHttpProxyConfig()
	c.Check(cfg2.Policy, Equals, config.Deny)
	c.Check(cfg2.Rules[1].Dst, DeepEquals, []config.IPNet{
		ipNet("1.2.3.4/32"),
		ipNet("1.2.3.5/32"),
		ipNet("1.2.4.0/24"),
	})

	// Store the modified configuration
	config.SetHttpProxyConfig(cfg)

	// Reload configuration
	cfg3 := config.GetHttpProxyConfig()
	c.Check(cfg3.Policy, Equals, config.Allow)
	c.Check(cfg3.Rules[1].Dst, DeepEquals, []config.IPNet{})
}

func ipNet(addr string) config.IPNet {
	_, ipnet, err := net.ParseCIDR(addr)
	if err != nil {
		panic(err)
	}
	return config.IPNet{IPNet: *ipnet}
}
