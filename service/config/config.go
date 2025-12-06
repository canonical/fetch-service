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

package config

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
)

// ACL configuration

type ACLPolicy int

const (
	Allow ACLPolicy = iota
	Deny
)

const (
	aclConfigFile        = "acl.yaml"
	inspectorsConfigFile = "inspectors.yaml"
)

func (t ACLPolicy) MarshalYAML() (interface{}, error) {
	switch t {
	case Allow:
		return "allow", nil
	case Deny:
		return "deny", nil
	default:
		return nil, errors.New("invalid ACL policy")
	}
}

func (t *ACLPolicy) UnmarshalYAML(unmarshal func(v interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	switch string(s) {
	case "allow", `"allow"`:
		*t = Allow
		return nil
	case "deny", `"deny"`:
		*t = Deny
		return nil
	default:
		return errors.New("invalid ACL policy")
	}

}

func (t ACLPolicy) String() string {
	switch t {
	case Allow:
		return "allow"
	case Deny:
		return "deny"
	default:
		return ""
	}
}

type IPNet struct {
	net.IPNet
}

func (t IPNet) MarshalYAML() (interface{}, error) {
	return t.String(), nil
}

func (t *IPNet) UnmarshalYAML(unmarshal func(v interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	// Ensure CIDR format.
	if strings.Contains(s, ".") && !strings.Contains(s, "/") {
		s += "/32"
	} else if strings.Contains(s, ":") && !strings.Contains(s, "/") {
		s += "/128"
	}

	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return err
	}
	*t = IPNet{*ipnet}
	return nil
}

type Rule struct {
	Dst    []IPNet   `yaml:"dst"`
	Access ACLPolicy `yaml:"access"`
}

type HTTPProxyConfig struct {
	Policy ACLPolicy `yaml:"policy"`
	Rules  []Rule    `yaml:"rules"`
}

type ACLConfig struct {
	HTTPProxy HTTPProxyConfig `yaml:"http-proxy"`
}

var (
	globalACLConfig     ACLConfig
	globalACLConfigLock sync.Mutex
)

func GetHTTPProxyConfig() HTTPProxyConfig {
	globalACLConfigLock.Lock()
	defer globalACLConfigLock.Unlock()

	cfg := HTTPProxyConfig{
		Policy: globalACLConfig.HTTPProxy.Policy,
		Rules:  make([]Rule, len(globalACLConfig.HTTPProxy.Rules)),
	}

	copy(cfg.Rules, globalACLConfig.HTTPProxy.Rules)

	return cfg
}

func SetHTTPProxyConfig(cfg HTTPProxyConfig) {
	globalACLConfigLock.Lock()
	defer globalACLConfigLock.Unlock()

	globalACLConfig.HTTPProxy.Policy = cfg.Policy
	globalACLConfig.HTTPProxy.Rules = make([]Rule, len(cfg.Rules))
	copy(globalACLConfig.HTTPProxy.Rules, cfg.Rules)

	logger.Infof("Proxy configuration updated: %d dst rules, default policy: %s",
		len(cfg.Rules), cfg.Policy.String())
}

func LoadHTTPProxyRules(cfgdir string) error {
	cfgfile := filepath.Join(cfgdir, aclConfigFile)
	if _, err := os.Stat(cfgfile); err != nil {
		return err
	}
	logger.Infof("Load proxy rules from %s", cfgfile)

	f, err := os.Open(cfgfile)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	cfg, err := decodeHTTPProxyRules(f)
	if err != nil {
		return err
	}

	// The configuration is only updated if the configuration file
	// is correctly parsed.
	SetHTTPProxyConfig(cfg.HTTPProxy)

	return nil
}

func decodeHTTPProxyRules(r io.Reader) (ACLConfig, error) {
	var cfg ACLConfig
	dec := yaml.NewDecoder(r)
	if err := dec.Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func UpdateConfig(optype string, dryRun bool, payload []byte, cfgdir string) error {
	r := bytes.NewReader(payload)

	switch optype {
	case "acl":
		cfg, err := decodeHTTPProxyRules(r)
		if err != nil {
			return err
		}
		if !dryRun {
			SetHTTPProxyConfig(cfg.HTTPProxy)

			// Overwrite the configuration file only if the data is valid
			// and we're not in a dry run.
			if err := updateConfigFile(cfgdir, aclConfigFile, payload); err != nil {
				return err
			}
			logger.Infof("config: write ACL configuration file: %s", filepath.Join(cfgdir, aclConfigFile))
		}
	case "inspectors":
		cfg, err := decodeInspectorsConfig(r)
		if err != nil {
			return err
		}
		if !dryRun {
			SetInspectorsConfig(cfg)

			// Overwrite the configuration file only if the data is valid
			// and we're not in a dry run.
			if err := updateConfigFile(cfgdir, inspectorsConfigFile, payload); err != nil {
				return err
			}
			logger.Infof("config: write inspectors configuration file: %s", filepath.Join(cfgdir, inspectorsConfigFile))
		}
	}
	return nil
}

func updateConfigFile(cfgdir, filename string, payload []byte) error {
	if err := os.MkdirAll(cfgdir, 0755); err != nil {
		return err
	}

	cfgfile := filepath.Join(cfgdir, filename)
	tmpfile := cfgfile + ".new"

	if err := os.WriteFile(tmpfile, payload, 0644); err != nil {
		return nil
	}

	if err := os.Rename(tmpfile, cfgfile); err != nil {
		return err
	}

	return nil
}

// Inspector configuration

var (
	globalInspectorsConfig     InspectorsConfig
	globalInspectorsConfigLock sync.Mutex
)

type InspectorsConfig struct {
	Apt    apt_cfg.AptInspectorConfig       `yaml:"apt"`
	Git    git_cfg.GitInspectorConfig       `yaml:"git"`
	Crafts crafts_cfg.CraftsInspectorConfig `yaml:"crafts"`
	Chisel chisel_cfg.ChiselInspectorConfig `yaml:"chisel"`
	Snap   snap_cfg.SnapInspectorConfig     `yaml:"snap"`
	Store  store_cfg.StoreInspectorConfig   `yaml:"store"`
	BldBin bldbin_cfg.BldBinInspectorConfig `yaml:"bldbin"`
}

func LoadInspectorsConfig(cfgdir string) error {
	cfgfile := filepath.Join(cfgdir, inspectorsConfigFile)
	if _, err := os.Stat(cfgfile); err != nil {
		return err
	}

	logger.Infof("Load inspectors configuration from %s", cfgfile)

	f, err := os.Open(cfgfile)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	cfg, err := decodeInspectorsConfig(f)
	if err != nil {
		return err
	}

	logger.Debugf("Inspectors configuration: %+v", cfg)

	// The configuration is only updated if the configuration file
	// is correctly parsed.
	SetInspectorsConfig(cfg)

	logger.Info("Inspectors configuration updated")

	return nil
}

func decodeInspectorsConfig(r io.Reader) (InspectorsConfig, error) {
	var cfg InspectorsConfig
	dec := yaml.NewDecoder(r)
	if err := dec.Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func GetInspectorsConfig() InspectorsConfig {
	cfg := InspectorsConfig{
		Apt: apt_cfg.AptInspectorConfig{
			Repositories: map[string]apt_cfg.AptInspectorConfigRepository{},
		},
	}

	globalInspectorsConfigLock.Lock()
	defer globalInspectorsConfigLock.Unlock()

	for k, v := range globalInspectorsConfig.Apt.Repositories {
		cfg.Apt.Repositories[k] = apt_cfg.AptInspectorConfigRepository{
			URLs:         v.URLs,
			Suites:       v.Suites,
			Components:   v.Components,
			PublicKey:    v.PublicKey,
			BaseURLAlias: v.BaseURLAlias,
		}
	}

	cfg.Git.URLs = make([]glob.Glob, len(globalInspectorsConfig.Git.URLs))
	copy(cfg.Git.URLs, globalInspectorsConfig.Git.URLs)

	cfg.Crafts.URLs = make([]glob.Glob, len(globalInspectorsConfig.Crafts.URLs))
	copy(cfg.Crafts.URLs, globalInspectorsConfig.Crafts.URLs)

	cfg.Store.URLs = make([]glob.Glob, len(globalInspectorsConfig.Store.URLs))
	copy(cfg.Store.URLs, globalInspectorsConfig.Store.URLs)

	cfg.BldBin.URLs = make([]glob.Glob, len(globalInspectorsConfig.BldBin.URLs))
	copy(cfg.BldBin.URLs, globalInspectorsConfig.BldBin.URLs)

	cfg.Chisel.URLs = make([]glob.Glob, len(globalInspectorsConfig.Chisel.URLs))
	copy(cfg.Chisel.URLs, globalInspectorsConfig.Chisel.URLs)

	cfg.Snap.SnapDeclarationFilter = make([]snap_cfg.AssertionFilter, len(globalInspectorsConfig.Snap.SnapDeclarationFilter))
	for i, v := range globalInspectorsConfig.Snap.SnapDeclarationFilter {
		newFilterValue := make([]string, len(v.Value))
		copy(newFilterValue, v.Value)
		cfg.Snap.SnapDeclarationFilter[i] = snap_cfg.AssertionFilter{
			Name:  v.Name,
			Value: newFilterValue,
		}
	}

	return cfg
}

func SetInspectorsConfig(cfg InspectorsConfig) {
	globalInspectorsConfigLock.Lock()
	defer globalInspectorsConfigLock.Unlock()
	globalInspectorsConfig = cfg
}

// Override inspectors configuration

type OverrideInspectorsConfig struct {
	Apt    *apt_cfg.AptInspectorConfig       `yaml:"apt" json:"apt"`
	Git    *git_cfg.GitInspectorConfig       `yaml:"git" json:"git"`
	Crafts *crafts_cfg.CraftsInspectorConfig `yaml:"crafts" json:"crafts"`
	Chisel *chisel_cfg.ChiselInspectorConfig `yaml:"chisel" json:"chisel"`
	Snap   *snap_cfg.SnapInspectorConfig     `yaml:"snap" json:"snap"`
	Store  *store_cfg.StoreInspectorConfig   `yaml:"store" json:"store"`
	BldBin *bldbin_cfg.BldBinInspectorConfig `yaml:"bldbin" json:"bldbin"`
}

func LoadOverrideInspectorsConfig(cfgdir string) error {
	cfgfile := filepath.Join(cfgdir, inspectorsConfigFile)
	if _, err := os.Stat(cfgfile); err != nil {
		return err
	}

	logger.Infof("Load inspectors configuration override from %s", cfgfile)

	f, err := os.Open(cfgfile)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	overrideCfg, err := decodeOverrideInspectorsConfig(f)
	if err != nil {
		return err
	}

	// The configuration is only updated if the configuration file
	// is correctly parsed.
	cfg := GetInspectorsConfig()
	cfg.Combine(overrideCfg)
	SetInspectorsConfig(cfg)

	logger.Info("Inspectors configuration updated")

	return nil
}

func decodeOverrideInspectorsConfig(r io.Reader) (OverrideInspectorsConfig, error) {
	var cfg OverrideInspectorsConfig
	dec := yaml.NewDecoder(r)
	if err := dec.Decode(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Combine applies inspectors configuration on an existing one.
// The strategy is to replace configurations on a per-inspector basis.
func (i *InspectorsConfig) Combine(c OverrideInspectorsConfig) {
	if c.Apt != nil {
		i.Apt = apt_cfg.AptInspectorConfig{
			Repositories: map[string]apt_cfg.AptInspectorConfigRepository{},
		}
		for k, v := range c.Apt.Repositories {
			i.Apt.Repositories[k] = apt_cfg.AptInspectorConfigRepository{
				URLs:         v.URLs,
				Suites:       v.Suites,
				Components:   v.Components,
				PublicKey:    v.PublicKey,
				BaseURLAlias: v.BaseURLAlias,
			}
		}
	}
	if c.Git != nil {
		i.Git.URLs = make([]glob.Glob, len(c.Git.URLs))
		copy(i.Git.URLs, c.Git.URLs)
	}
	if c.Crafts != nil {
		i.Crafts.URLs = make([]glob.Glob, len(c.Crafts.URLs))
		copy(i.Crafts.URLs, c.Crafts.URLs)
	}
	if c.Chisel != nil {
		i.Chisel.URLs = make([]glob.Glob, len(c.Chisel.URLs))
		copy(i.Chisel.URLs, c.Chisel.URLs)
	}
	if c.Snap != nil {
		i.Snap.SnapDeclarationFilter = make([]snap_cfg.AssertionFilter, len(c.Snap.SnapDeclarationFilter))
		copy(i.Snap.SnapDeclarationFilter, c.Snap.SnapDeclarationFilter)
	}
	if c.Store != nil {
		i.Store.URLs = make([]glob.Glob, len(c.Store.URLs))
		copy(i.Store.URLs, c.Store.URLs)
	}
	if c.BldBin != nil {
		i.BldBin.URLs = make([]glob.Glob, len(c.BldBin.URLs))
		copy(i.BldBin.URLs, c.BldBin.URLs)
	}
}
