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

package acl_test

import (
	"net"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/proxy/acl"
	"github.com/canonical/fetch-service/service/config"
)

func Test(t *testing.T) { TestingT(t) }

type aclSuite struct{}

func (t *aclSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&aclSuite{})

var proxyConfig = config.HTTPProxyConfig{
	Policy: config.Allow,
	Rules: []config.Rule{
		{
			Dst: []config.IPNet{
				ipNet("10.1.5.153/32"),
				ipNet("ff80::4:20:30/128"),
			},
			Access: config.Deny,
		},
		{
			Dst: []config.IPNet{
				ipNet("10.1.5.0/24"),
				ipNet("ff80::4:20:0/112"),
			},
			Access: config.Allow,
		},
		{
			Dst: []config.IPNet{
				ipNet("10.1.0.0/16"),
				ipNet("ff80::4:0:0/96"),
			},
			Access: config.Deny,
		},
	},
}

func (t *aclSuite) TestAllowed(c *C) {
	restore := setProxyConfig(proxyConfig)
	defer restore()

	for _, tc := range []struct {
		addr    string
		allowed bool
	}{
		// IPv4 addresses
		{"0.0.0.0", true},     // doesn't match any rule, use default policy
		{"127.0.0.1", true},   //
		{"10.0.0.0", true},    //
		{"10.1.5.153", false}, // matches rule #1
		{"10.1.5.152", true},  // matches rule #2
		{"10.1.2.3", false},   // matches rule #3

		// IPv6 addresses
		{"::", true},             // doesn't match any rule, use default policy
		{"::1", true},            //
		{"ff80::1", true},        //
		{"ff80::4:20:30", false}, // matches rule #1
		{"ff80::4:20:3", true},   // matches rule #2
		{"ff80::4:1:2", false},   // matches rule #3

		// Invalid addresses
		{"10.1.5.256", false},  // invalid IPv4 address
		{"[ff80::5.5]", false}, // invalid IPv6 address
		{"invalid", false},     // invalid address
		{"[invalid]", false},   // invalid address
	} {
		ip := net.ParseIP(tc.addr)
		c.Check(acl.Allowed(ip), Equals, tc.allowed)
	}

}

func (t *aclSuite) TestDefaultPolicy(c *C) {
	for _, tc := range []struct {
		policy  config.ACLPolicy
		allowed bool
	}{
		{config.Allow, true},
		{config.Deny, false},
	} {
		cfg := config.HTTPProxyConfig{
			Policy: tc.policy,
			Rules: []config.Rule{
				config.Rule{
					Dst:    []config.IPNet{ipNet("10.0.2.0/24")},
					Access: config.Allow,
				},
				config.Rule{
					Dst:    []config.IPNet{ipNet("10.0.3.0/24")},
					Access: config.Deny,
				},
			},
		}

		restore := setProxyConfig(cfg)
		defer restore()

		ip := net.ParseIP("10.0.2.5")
		c.Check(acl.Allowed(ip), Equals, true) // matches rule #1

		ip = net.ParseIP("10.0.3.5")
		c.Check(acl.Allowed(ip), Equals, false) // matches rule #2

		ip = net.ParseIP("10.0.5.5")
		c.Check(acl.Allowed(ip), Equals, tc.allowed) // use default policy
	}
}

func setProxyConfig(cfg config.HTTPProxyConfig) func() {
	old := config.GetHTTPProxyConfig()
	config.SetHTTPProxyConfig(cfg)
	return func() {
		config.SetHTTPProxyConfig(old)
	}
}

func ipNet(addr string) config.IPNet {
	_, ipnet, err := net.ParseCIDR(addr)
	if err != nil {
		panic(err)
	}
	return config.IPNet{IPNet: *ipnet}
}
