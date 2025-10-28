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

package config_test

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v3"

	"github.com/canonical/fetch-service/glob"
	apt_cfg "github.com/canonical/fetch-service/inspectors/apt/config"
	bldbin_cfg "github.com/canonical/fetch-service/inspectors/bldbin/config"
	chisel_cfg "github.com/canonical/fetch-service/inspectors/chisel/config"
	crafts_cfg "github.com/canonical/fetch-service/inspectors/craft/config"
	git_cfg "github.com/canonical/fetch-service/inspectors/git/config"
	snap_cfg "github.com/canonical/fetch-service/inspectors/snap/config"
	store_cfg "github.com/canonical/fetch-service/inspectors/store/config"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/testutils"
)

func Test(t *testing.T) { TestingT(t) }

type configSuite struct{}

func (t *configSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&configSuite{})

type marshalACLPolicyTest struct {
	value     config.ACLPolicy // The ACL configuration value
	errorMsg  string           // The error message, if any
	marshaled string           // The expected marshaled policy string
}

var marshalACLPolicyTests = []marshalACLPolicyTest{{
	value:     config.Allow,
	errorMsg:  "",
	marshaled: "foo: allow\n",
}, {
	value:     config.Deny,
	errorMsg:  "",
	marshaled: "foo: deny\n",
}, {
	value:     42,
	errorMsg:  "invalid ACL policy",
	marshaled: "",
}}

func (t *configSuite) TestMarshalACLPolicy(c *C) {
	type FooStruct struct {
		Foo config.ACLPolicy `yaml:"foo"`
	}

	for _, tc := range marshalACLPolicyTests {
		d, err := yaml.Marshal(FooStruct{Foo: tc.value})
		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(string(d), Equals, tc.marshaled)
		} else {
			c.Assert(err.Error(), Equals, tc.errorMsg)
		}
	}
}

type unmarshalACLPolicyTest struct {
	marshaled string           // The marshaled string
	errorMsg  string           // The error message, if any
	value     config.ACLPolicy // The expected ACL policy value
}

var unmarshalACLPolicyTests = []unmarshalACLPolicyTest{{
	marshaled: "foo: allow",
	errorMsg:  "",
	value:     config.Allow,
}, {
	marshaled: "foo: deny",
	errorMsg:  "",
	value:     config.Deny,
}, {
	marshaled: "foo: invalid",
	errorMsg:  "invalid ACL policy",
	value:     0,
}}

func (t *configSuite) TestUnmarshalACLPolicy(c *C) {
	type FooStruct struct {
		Foo config.ACLPolicy `yaml:"foo"`
	}

	for _, tc := range unmarshalACLPolicyTests {
		var foo FooStruct
		err := yaml.Unmarshal([]byte(tc.marshaled), &foo)
		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(foo.Foo, Equals, tc.value)
		} else {
			c.Assert(err.Error(), Equals, tc.errorMsg)
		}
	}
}

func (t *configSuite) TestACLPolicyString(c *C) {
	c.Assert(config.Allow.String(), Equals, "allow")
	c.Assert(config.Deny.String(), Equals, "deny")
}

type marshalIPNetTest struct {
	value     string // The IP address to marshal
	errorMsg  string // The error message, if any
	marshaled string // The expected marshaled IP address
}

var marshalIPNetTests = []marshalIPNetTest{{
	value:     "10.42.42.0/24",
	errorMsg:  "",
	marshaled: "foo: 10.42.42.0/24\n",
}, {
	value:     "10.42.42.10/24",
	errorMsg:  "",
	marshaled: "foo: 10.42.42.0/24\n",
}, {
	value:     "::1/128",
	errorMsg:  "",
	marshaled: "foo: ::1/128\n",
}, {
	value:     "0:0:0:0::1/128",
	errorMsg:  "",
	marshaled: "foo: ::1/128\n",
}, {
	value:     "ff80::1/128",
	errorMsg:  "",
	marshaled: "foo: ff80::1/128\n",
}}

func (t *configSuite) TestMarshalIPNet(c *C) {
	type FooStruct struct {
		Foo config.IPNet `yaml:"foo"`
	}

	for _, tc := range marshalIPNetTests {
		_, ipnet, err := net.ParseCIDR(tc.value)
		c.Assert(err, IsNil)

		d, err := yaml.Marshal(FooStruct{Foo: config.IPNet{*ipnet}})
		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			c.Check(string(d), Equals, tc.marshaled)
		} else {
			c.Assert(err.Error(), Equals, tc.errorMsg)
		}
	}
}

type unmarshalIPNetTest struct {
	marshaled string // The marshaled IP string
	errorMsg  string // The error message, if any
	value     string // The expected IP value
}

var unmarshalIPNetTests = []unmarshalIPNetTest{{
	marshaled: "foo: 10.42.42.0/24",
	errorMsg:  "",
	value:     "10.42.42.0/24",
}, {
	marshaled: "foo: 10.42.42.10",
	errorMsg:  "",
	value:     "10.42.42.10/32",
}, {
	marshaled: "foo: ::1/128",
	errorMsg:  "",
	value:     "::1/128",
}, {
	marshaled: "foo: 0:0:0:0::1/128",
	errorMsg:  "",
	value:     "::1/128",
}, {
	marshaled: "foo: ff80::1/64",
	errorMsg:  "",
	value:     "ff80::/64",
}, {
	marshaled: "foo: FF80::1/64",
	errorMsg:  "",
	value:     "ff80::/64",
}, {
	marshaled: "foo: xxx",
	errorMsg:  "invalid CIDR address: xxx",
	value:     "",
}}

func (t *configSuite) TestUnmarshalIPNet(c *C) {
	type FooStruct struct {
		Foo config.IPNet `yaml:"foo"`
	}

	for _, tc := range unmarshalIPNetTests {
		var foo FooStruct
		err := yaml.Unmarshal([]byte(tc.marshaled), &foo)
		if tc.errorMsg == "" {
			c.Assert(err, IsNil)
			c.Check(foo.Foo.String(), Equals, tc.value)
		} else {
			c.Assert(err.Error(), Equals, tc.errorMsg)
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

func (t *configSuite) TestGetSetHTTPProxyConfig(c *C) {
	dir := c.MkDir()
	cfgFile := filepath.Join(dir, "acl.yaml")
	err := os.WriteFile(cfgFile, []byte(proxyConfig), 0644)
	c.Assert(err, IsNil)

	// Load rules from file
	err = config.LoadHTTPProxyRules(dir)
	c.Assert(err, IsNil)

	cfg := config.GetHTTPProxyConfig()
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

	cfg2 := config.GetHTTPProxyConfig()
	c.Check(cfg2.Policy, Equals, config.Deny)
	c.Check(cfg2.Rules[1].Dst, DeepEquals, []config.IPNet{
		ipNet("1.2.3.4/32"),
		ipNet("1.2.3.5/32"),
		ipNet("1.2.4.0/24"),
	})

	// Store the modified configuration
	config.SetHTTPProxyConfig(cfg)

	// Reload configuration
	cfg3 := config.GetHTTPProxyConfig()
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

var inspectorsConfig = `
git:
  urls:
    - https://git.test:443/**

crafts:
  urls:
    - https://sourcecraft.test:443/**

chisel:
  urls:
    - https://codeload.github.com:443/canonical/chisel-releases/**

snap:
  snap-declaration:
    - name: publisher-id
      value: [canonical]

apt:
  repositories:
    default:
      urls:
        - http://archive.ubuntu.com/ubuntu
        - http://*.archive.ubuntu.com/ubuntu
      suites:
        - "*"
      components:
        - "*"
      public-key: |
        -----BEGIN PGP PUBLIC KEY BLOCK-----

        mQINBFufwdoBEADv/Gxytx/LcSXYuM0MwKojbBye81s0G1nEx+lz6VAUpIUZnbkq
        dXBHC+dwrGS/CeeLuAjPRLU8AoxE/jjvZVp8xFGEWHYdklqXGZ/gJfP5d3fIUBtZ
        HZEJl8B8m9pMHf/AQQdsC+YzizSG5t5Mhnotw044LXtdEEkx2t6Jz0OGrh+5Ioxq
        X7pZiq6Cv19BohaUioKMdp7ES6RYfN7ol6HSLFlrMXtVfh/ijpN9j3ZhVGVeRC8k
        KHQsJ5PkIbmvxBiUh7SJmfZUx0IQhNMaDHXfdZAGNtnhzzNReb1FqNLSVkrS/Pns
        AQzMhG1BDm2VOSF64jebKXffFqM5LXRQTeqTLsjUbbrqR6s/GCO8UF7jfUj6I7ta
        LygmsHO/JD4jpKRC0gbpUBfaiJyLvuepx3kWoqL3sN0LhlMI80+fA7GTvoOx4tpq
        VlzlE6TajYu+jfW3QpOFS5ewEMdL26hzxsZg/geZvTbArcP+OsJKRmhv4kNo6Ayd
        yHQ/3ZV/f3X9mT3/SPLbJaumkgp3Yzd6t5PeBu+ZQk/mN5WNNuaihNEV7llb1Zhv
        Y0Fxu9BVd/BNl0rzuxp3rIinB2TX2SCg7wE5xXkwXuQ/2eTDE0v0HlGntkuZjGow
        DZkxHZQSxZVOzdZCRVaX/WEFLpKa2AQpw5RJrQ4oZ/OfifXyJzP27o03wQARAQAB
        tEJVYnVudHUgQXJjaGl2ZSBBdXRvbWF0aWMgU2lnbmluZyBLZXkgKDIwMTgpIDxm
        dHBtYXN0ZXJAdWJ1bnR1LmNvbT6JAjgEEwEKACIFAlufwdoCGwMGCwkIBwMCBhUI
        AgkKCwQWAgMBAh4BAheAAAoJEIcZINGZG8k8LHMQAKS2cnxz/5WaoCOWArf5g6UH
        beOCgc5DBm0hCuFDZWWv427aGei3CPuLw0DGLCXZdyc5dqE8mvjMlOmmAKKlj1uG
        g3TYCbQWjWPeMnBPZbkFgkZoXJ7/6CB7bWRht1sHzpt1LTZ+SYDwOwJ68QRp7DRa
        Zl9Y6QiUbeuhq2DUcTofVbBxbhrckN4ZteLvm+/nG9m/ciopc66LwRdkxqfJ32Cy
        q+1TS5VaIJDG7DWziG+Kbu6qCDM4QNlg3LH7p14CrRxAbc4lvohRgsV4eQqsIcdF
        kuVY5HPPj2K8TqpY6STe8Gh0aprG1RV8ZKay3KSMpnyV1fAKn4fM9byiLzQAovC0
        LZ9MMMsrAS/45AvC3IEKSShjLFn1X1dRCiO6/7jmZEoZtAp53hkf8SMBsi78hVNr
        BumZwfIdBA1v22+LY4xQK8q4XCoRcA9G+pvzU9YVW7cRnDZZGl0uwOw7z9PkQBF5
        KFKjWDz4fCk+K6+YtGpovGKekGBb8I7EA6UpvPgqA/QdI0t1IBP0N06RQcs1fUaA
        QEtz6DGy5zkRhR4pGSZn+dFET7PdAjEK84y7BdY4t+U1jcSIvBj0F2B7LwRL7xGp
        SpIKi/ekAXLs117bvFHaCvmUYN7JVp1GMmVFxhIdx6CFm3fxG8QjNb5tere/YqK+
        uOgcXny1UlwtCUzlrSaP
        =9AdM
        -----END PGP PUBLIC KEY BLOCK-----
`

func (t *configSuite) TestGetSetInspectorsConfig(c *C) {
	dir := c.MkDir()
	cfgFile := filepath.Join(dir, "inspectors.yaml")
	err := os.WriteFile(cfgFile, []byte(inspectorsConfig), 0644)
	c.Assert(err, IsNil)

	// Load rules from file
	err = config.LoadInspectorsConfig(dir)
	c.Assert(err, IsNil)

	cfg := config.GetInspectorsConfig()
	c.Check(cfg.Git.Urls, HasLen, 1)
	c.Check(cfg.Git.Urls[0], DeepEquals, glob.MustCompile("https://git.test:443/**"))

	c.Check(cfg.Crafts.Urls, HasLen, 1)
	c.Check(cfg.Crafts.Urls[0], DeepEquals, glob.MustCompile("https://sourcecraft.test:443/**"))

	c.Check(cfg.Chisel.Urls, HasLen, 1)
	c.Check(cfg.Chisel.Urls[0], DeepEquals,
		glob.MustCompile("https://codeload.github.com:443/canonical/chisel-releases/**"))

	c.Check(cfg.Apt.Repositories, DeepEquals, map[string]apt_cfg.AptInspectorConfigRepository{
		"default": {
			Urls: []glob.Glob{
				glob.MustCompile("http://archive.ubuntu.com/ubuntu"),
				glob.MustCompile("http://*.archive.ubuntu.com/ubuntu"),
			},
			Suites: []glob.Glob{
				glob.MustCompile("*"),
			},
			Components: []glob.Glob{
				glob.MustCompile("*"),
			},
			PublicKey: `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBFufwdoBEADv/Gxytx/LcSXYuM0MwKojbBye81s0G1nEx+lz6VAUpIUZnbkq
dXBHC+dwrGS/CeeLuAjPRLU8AoxE/jjvZVp8xFGEWHYdklqXGZ/gJfP5d3fIUBtZ
HZEJl8B8m9pMHf/AQQdsC+YzizSG5t5Mhnotw044LXtdEEkx2t6Jz0OGrh+5Ioxq
X7pZiq6Cv19BohaUioKMdp7ES6RYfN7ol6HSLFlrMXtVfh/ijpN9j3ZhVGVeRC8k
KHQsJ5PkIbmvxBiUh7SJmfZUx0IQhNMaDHXfdZAGNtnhzzNReb1FqNLSVkrS/Pns
AQzMhG1BDm2VOSF64jebKXffFqM5LXRQTeqTLsjUbbrqR6s/GCO8UF7jfUj6I7ta
LygmsHO/JD4jpKRC0gbpUBfaiJyLvuepx3kWoqL3sN0LhlMI80+fA7GTvoOx4tpq
VlzlE6TajYu+jfW3QpOFS5ewEMdL26hzxsZg/geZvTbArcP+OsJKRmhv4kNo6Ayd
yHQ/3ZV/f3X9mT3/SPLbJaumkgp3Yzd6t5PeBu+ZQk/mN5WNNuaihNEV7llb1Zhv
Y0Fxu9BVd/BNl0rzuxp3rIinB2TX2SCg7wE5xXkwXuQ/2eTDE0v0HlGntkuZjGow
DZkxHZQSxZVOzdZCRVaX/WEFLpKa2AQpw5RJrQ4oZ/OfifXyJzP27o03wQARAQAB
tEJVYnVudHUgQXJjaGl2ZSBBdXRvbWF0aWMgU2lnbmluZyBLZXkgKDIwMTgpIDxm
dHBtYXN0ZXJAdWJ1bnR1LmNvbT6JAjgEEwEKACIFAlufwdoCGwMGCwkIBwMCBhUI
AgkKCwQWAgMBAh4BAheAAAoJEIcZINGZG8k8LHMQAKS2cnxz/5WaoCOWArf5g6UH
beOCgc5DBm0hCuFDZWWv427aGei3CPuLw0DGLCXZdyc5dqE8mvjMlOmmAKKlj1uG
g3TYCbQWjWPeMnBPZbkFgkZoXJ7/6CB7bWRht1sHzpt1LTZ+SYDwOwJ68QRp7DRa
Zl9Y6QiUbeuhq2DUcTofVbBxbhrckN4ZteLvm+/nG9m/ciopc66LwRdkxqfJ32Cy
q+1TS5VaIJDG7DWziG+Kbu6qCDM4QNlg3LH7p14CrRxAbc4lvohRgsV4eQqsIcdF
kuVY5HPPj2K8TqpY6STe8Gh0aprG1RV8ZKay3KSMpnyV1fAKn4fM9byiLzQAovC0
LZ9MMMsrAS/45AvC3IEKSShjLFn1X1dRCiO6/7jmZEoZtAp53hkf8SMBsi78hVNr
BumZwfIdBA1v22+LY4xQK8q4XCoRcA9G+pvzU9YVW7cRnDZZGl0uwOw7z9PkQBF5
KFKjWDz4fCk+K6+YtGpovGKekGBb8I7EA6UpvPgqA/QdI0t1IBP0N06RQcs1fUaA
QEtz6DGy5zkRhR4pGSZn+dFET7PdAjEK84y7BdY4t+U1jcSIvBj0F2B7LwRL7xGp
SpIKi/ekAXLs117bvFHaCvmUYN7JVp1GMmVFxhIdx6CFm3fxG8QjNb5tere/YqK+
uOgcXny1UlwtCUzlrSaP
=9AdM
-----END PGP PUBLIC KEY BLOCK-----
`,
		},
	})

	// Verify that loaded config is a copy
	cfg.Git.Urls = []glob.Glob{}
	cfg.Crafts.Urls = []glob.Glob{}
	cfg.Chisel.Urls = []glob.Glob{}
	entry, ok := cfg.Apt.Repositories["default"]
	c.Assert(ok, Equals, true)
	entry.Urls = []glob.Glob{
		glob.MustCompile("a"),
		glob.MustCompile("b"),
		glob.MustCompile("c"),
	}
	cfg.Apt.Repositories["default"] = entry
	cfg.Apt.Repositories["extra"] = apt_cfg.AptInspectorConfigRepository{}

	cfg2 := config.GetInspectorsConfig()
	c.Check(cfg2.Git.Urls, HasLen, 1)
	c.Check(cfg2.Crafts.Urls, HasLen, 1)
	c.Check(cfg2.Chisel.Urls, HasLen, 1)
	c.Check(cfg2.Apt.Repositories, HasLen, 1)
	c.Check(cfg2.Apt.Repositories["default"].Urls, HasLen, 2)

	// Store the modified configuration
	config.SetInspectorsConfig(cfg)

	// Reload configuration
	cfg3 := config.GetInspectorsConfig()
	c.Check(cfg3.Git.Urls, HasLen, 0)
	c.Check(cfg3.Crafts.Urls, HasLen, 0)
	c.Check(cfg3.Chisel.Urls, HasLen, 0)
	c.Check(cfg3.Apt.Repositories["default"].Urls, HasLen, 3)
	c.Check(cfg3.Apt.Repositories["extra"].Urls, HasLen, 0)

}

var proxyRulesContent = testutils.Reindent(`
	http-proxy:
	  policy: allow
	  rules:
	    - dst: [ 1.2.3.4/16 ]
	      access: deny
`)

func (t *configSuite) TestLoadHTTPProxyRules(c *C) {
	dir := c.MkDir()

	emptyConfig := config.HTTPProxyConfig{Policy: config.Deny, Rules: []config.Rule{}}
	config.SetHTTPProxyConfig(emptyConfig)

	err := os.WriteFile(filepath.Join(dir, "acl.yaml"), proxyRulesContent, 0644)
	c.Assert(err, IsNil)

	err = config.LoadHTTPProxyRules(dir)
	c.Assert(err, IsNil)
	cfg := config.GetHTTPProxyConfig()
	c.Assert(cfg.Policy, Equals, config.Allow)
	c.Assert(cfg.Rules, DeepEquals, []config.Rule{{
		Dst:    []config.IPNet{{net.IPNet{IP: net.IP{1, 2, 0, 0}, Mask: net.IPMask{255, 255, 0, 0}}}},
		Access: config.Deny,
	}})
}

func (t *configSuite) TestLoadHTTPProxyRulesMissing(c *C) {
	dir := c.MkDir()

	emptyConfig := config.HTTPProxyConfig{Policy: config.Deny, Rules: []config.Rule{}}
	config.SetHTTPProxyConfig(emptyConfig)

	err := config.LoadHTTPProxyRules(dir)
	c.Assert(errors.Is(err, os.ErrNotExist), Equals, true)
}

var inspectorsConfigContent = testutils.Reindent(`
	apt:
	  repositories:
	    default:
	      urls:
	        - http://archive.ubuntu.com/ubuntu
`)

func (t *configSuite) TestLoadInspectorsConfig(c *C) {
	dir := c.MkDir()

	emptyConfig := config.InspectorsConfig{}
	config.SetInspectorsConfig(emptyConfig)

	err := os.WriteFile(filepath.Join(dir, "inspectors.yaml"), inspectorsConfigContent, 0644)
	c.Assert(err, IsNil)

	err = config.LoadInspectorsConfig(dir)
	c.Assert(err, IsNil)
	cfg := config.GetInspectorsConfig()
	c.Assert(cfg.Apt, DeepEquals, apt_cfg.AptInspectorConfig{
		Repositories: map[string]apt_cfg.AptInspectorConfigRepository{"default": {
			Urls: []glob.Glob{glob.MustCompile("http://archive.ubuntu.com/ubuntu")},
		}},
	})
}

func (t *configSuite) TestInspectorsConfigMissing(c *C) {
	dir := c.MkDir()

	emptyConfig := config.InspectorsConfig{}
	config.SetInspectorsConfig(emptyConfig)

	err := config.LoadInspectorsConfig(dir)
	c.Assert(errors.Is(err, os.ErrNotExist), Equals, true)
}

type combineInspectorsConfigTest struct {
	sessionInspectorsConfig config.SessionInspectorsConfig
	combined                config.InspectorsConfig
}

var combineInspectorsConfigTests = []combineInspectorsConfigTest{
	{
		sessionInspectorsConfig: config.SessionInspectorsConfig{
			Apt: &apt_cfg.AptInspectorConfig{
				Repositories: map[string]apt_cfg.AptInspectorConfigRepository{
					"another": {
						Urls:       []glob.Glob{glob.MustCompile("http://test.com/ubuntu")},
						Suites:     []glob.Glob{glob.MustCompile("*")},
						Components: []glob.Glob{glob.MustCompile("*")},
						PublicKey:  "",
					},
				},
			},
			Git: &git_cfg.GitInspectorConfig{
				Urls: []glob.Glob{
					glob.MustCompile("https://git.test2:443/**"),
				},
			},
			Crafts: &crafts_cfg.CraftsInspectorConfig{
				Urls: []glob.Glob{glob.MustCompile("https://git.launchpad.net:443/**")},
			},
			Chisel: &chisel_cfg.ChiselInspectorConfig{
				Urls: []glob.Glob{
					glob.MustCompile("https://codeload.another.com:443/canonical/chisel-releases/**"),
				},
			},
			Snap: &snap_cfg.SnapInspectorConfig{
				SnapDeclarationFilter: []snap_cfg.AssertionFilter{
					{
						Name:  "another-publisher-id",
						Value: []string{"not-canonical"},
					},
				},
			},
			Store: &store_cfg.StoreInspectorConfig{
				Urls: []glob.Glob{
					glob.MustCompile("https://a-store.com:443"),
				},
			},
			BldBin: &bldbin_cfg.BldBinInspectorConfig{
				Urls: []glob.Glob{
					glob.MustCompile("https://another.snapcraft.io:443/api/v1/bins/download/**"),
				},
			},
		},
		combined: config.InspectorsConfig{
			Apt: apt_cfg.AptInspectorConfig{
				Repositories: map[string]apt_cfg.AptInspectorConfigRepository{
					"another": {
						Urls:       []glob.Glob{glob.MustCompile("http://test.com/ubuntu")},
						Suites:     []glob.Glob{glob.MustCompile("*")},
						Components: []glob.Glob{glob.MustCompile("*")},
						PublicKey:  "",
					},
				},
			},
			Git: git_cfg.GitInspectorConfig{
				Urls: []glob.Glob{
					glob.MustCompile("https://git.test2:443/**"),
				},
			},
			Crafts: crafts_cfg.CraftsInspectorConfig{
				Urls: []glob.Glob{
					glob.MustCompile("https://git.launchpad.net:443/**"),
				},
			},
			Chisel: chisel_cfg.ChiselInspectorConfig{
				Urls: []glob.Glob{
					glob.MustCompile("https://codeload.another.com:443/canonical/chisel-releases/**"),
				},
			},
			Snap: snap_cfg.SnapInspectorConfig{
				SnapDeclarationFilter: []snap_cfg.AssertionFilter{
					{
						Name:  "another-publisher-id",
						Value: []string{"not-canonical"},
					},
				},
			},
			Store: store_cfg.StoreInspectorConfig{
				Urls: []glob.Glob{
					glob.MustCompile("https://a-store.com:443"),
				},
			},
			BldBin: bldbin_cfg.BldBinInspectorConfig{
				Urls: []glob.Glob{
					glob.MustCompile("https://another.snapcraft.io:443/api/v1/bins/download/**"),
				},
			},
		},
	},
	// override nothing when nothing is given
	{
		sessionInspectorsConfig: config.SessionInspectorsConfig{},
		combined: config.InspectorsConfig{
			Apt: apt_cfg.AptInspectorConfig{
				Repositories: map[string]apt_cfg.AptInspectorConfigRepository{
					"default": {
						Urls: []glob.Glob{
							glob.MustCompile("http://archive.ubuntu.com/ubuntu"),
							glob.MustCompile("http://*.archive.ubuntu.com/ubuntu"),
						},
						Suites: []glob.Glob{
							glob.MustCompile("*"),
						},
						Components: []glob.Glob{
							glob.MustCompile("*"),
						},
						PublicKey: `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBFufwdoBEADv/Gxytx/LcSXYuM0MwKojbBye81s0G1nEx+lz6VAUpIUZnbkq
dXBHC+dwrGS/CeeLuAjPRLU8AoxE/jjvZVp8xFGEWHYdklqXGZ/gJfP5d3fIUBtZ
HZEJl8B8m9pMHf/AQQdsC+YzizSG5t5Mhnotw044LXtdEEkx2t6Jz0OGrh+5Ioxq
X7pZiq6Cv19BohaUioKMdp7ES6RYfN7ol6HSLFlrMXtVfh/ijpN9j3ZhVGVeRC8k
KHQsJ5PkIbmvxBiUh7SJmfZUx0IQhNMaDHXfdZAGNtnhzzNReb1FqNLSVkrS/Pns
AQzMhG1BDm2VOSF64jebKXffFqM5LXRQTeqTLsjUbbrqR6s/GCO8UF7jfUj6I7ta
LygmsHO/JD4jpKRC0gbpUBfaiJyLvuepx3kWoqL3sN0LhlMI80+fA7GTvoOx4tpq
VlzlE6TajYu+jfW3QpOFS5ewEMdL26hzxsZg/geZvTbArcP+OsJKRmhv4kNo6Ayd
yHQ/3ZV/f3X9mT3/SPLbJaumkgp3Yzd6t5PeBu+ZQk/mN5WNNuaihNEV7llb1Zhv
Y0Fxu9BVd/BNl0rzuxp3rIinB2TX2SCg7wE5xXkwXuQ/2eTDE0v0HlGntkuZjGow
DZkxHZQSxZVOzdZCRVaX/WEFLpKa2AQpw5RJrQ4oZ/OfifXyJzP27o03wQARAQAB
tEJVYnVudHUgQXJjaGl2ZSBBdXRvbWF0aWMgU2lnbmluZyBLZXkgKDIwMTgpIDxm
dHBtYXN0ZXJAdWJ1bnR1LmNvbT6JAjgEEwEKACIFAlufwdoCGwMGCwkIBwMCBhUI
AgkKCwQWAgMBAh4BAheAAAoJEIcZINGZG8k8LHMQAKS2cnxz/5WaoCOWArf5g6UH
beOCgc5DBm0hCuFDZWWv427aGei3CPuLw0DGLCXZdyc5dqE8mvjMlOmmAKKlj1uG
g3TYCbQWjWPeMnBPZbkFgkZoXJ7/6CB7bWRht1sHzpt1LTZ+SYDwOwJ68QRp7DRa
Zl9Y6QiUbeuhq2DUcTofVbBxbhrckN4ZteLvm+/nG9m/ciopc66LwRdkxqfJ32Cy
q+1TS5VaIJDG7DWziG+Kbu6qCDM4QNlg3LH7p14CrRxAbc4lvohRgsV4eQqsIcdF
kuVY5HPPj2K8TqpY6STe8Gh0aprG1RV8ZKay3KSMpnyV1fAKn4fM9byiLzQAovC0
LZ9MMMsrAS/45AvC3IEKSShjLFn1X1dRCiO6/7jmZEoZtAp53hkf8SMBsi78hVNr
BumZwfIdBA1v22+LY4xQK8q4XCoRcA9G+pvzU9YVW7cRnDZZGl0uwOw7z9PkQBF5
KFKjWDz4fCk+K6+YtGpovGKekGBb8I7EA6UpvPgqA/QdI0t1IBP0N06RQcs1fUaA
QEtz6DGy5zkRhR4pGSZn+dFET7PdAjEK84y7BdY4t+U1jcSIvBj0F2B7LwRL7xGp
SpIKi/ekAXLs117bvFHaCvmUYN7JVp1GMmVFxhIdx6CFm3fxG8QjNb5tere/YqK+
uOgcXny1UlwtCUzlrSaP
=9AdM
-----END PGP PUBLIC KEY BLOCK-----
`,
					},
				},
			},
			Git: git_cfg.GitInspectorConfig{
				Urls: []glob.Glob{
					glob.MustCompile("https://git.test:443/**"),
				},
			},
			Crafts: crafts_cfg.CraftsInspectorConfig{
				Urls: []glob.Glob{
					glob.MustCompile("https://sourcecraft.test:443/**"),
				},
			},
			Chisel: chisel_cfg.ChiselInspectorConfig{
				Urls: []glob.Glob{
					glob.MustCompile("https://codeload.github.com:443/canonical/chisel-releases/**"),
				},
			},
			Snap: snap_cfg.SnapInspectorConfig{
				SnapDeclarationFilter: []snap_cfg.AssertionFilter{
					{
						Name:  "publisher-id",
						Value: []string{"canonical"},
					},
				},
			},
			Store: store_cfg.StoreInspectorConfig{
				Urls: []glob.Glob{},
			},
			BldBin: bldbin_cfg.BldBinInspectorConfig{
				Urls: []glob.Glob{},
			},
		},
	}}

func (t *configSuite) TestCombineInspectorsConfig(c *C) {
	dir := c.MkDir()
	cfgFile := filepath.Join(dir, "inspectors.yaml")
	err := os.WriteFile(cfgFile, []byte(inspectorsConfig), 0644)
	c.Assert(err, IsNil)

	// Load rules from file
	err = config.LoadInspectorsConfig(dir)
	c.Assert(err, IsNil)

	for _, tc := range combineInspectorsConfigTests {
		cfg := config.GetInspectorsConfig()
		cfg.Combine(tc.sessionInspectorsConfig)
		c.Check(cfg, DeepEquals, tc.combined)
	}
}
