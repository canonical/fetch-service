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

type HttpProxyConfig struct {
	Policy ACLPolicy `yaml:"policy"`
	Rules  []Rule    `yaml:"rules"`
}

type ACLConfig struct {
	HttpProxy HttpProxyConfig `yaml:"http-proxy"`
}

var (
	globalACLConfig     ACLConfig
	globalACLConfigLock sync.Mutex
)

func GetHttpProxyConfig() HttpProxyConfig {
	globalACLConfigLock.Lock()
	defer globalACLConfigLock.Unlock()

	cfg := HttpProxyConfig{
		Policy: globalACLConfig.HttpProxy.Policy,
		Rules:  make([]Rule, len(globalACLConfig.HttpProxy.Rules)),
	}

	copy(cfg.Rules, globalACLConfig.HttpProxy.Rules)

	return cfg
}

func SetHttpProxyConfig(cfg HttpProxyConfig) {
	globalACLConfigLock.Lock()
	defer globalACLConfigLock.Unlock()

	globalACLConfig.HttpProxy.Policy = cfg.Policy
	globalACLConfig.HttpProxy.Rules = make([]Rule, len(cfg.Rules))
	copy(globalACLConfig.HttpProxy.Rules, cfg.Rules)

	logger.Infof("Proxy configuration updated: %d dst rules, default policy: %s",
		len(cfg.Rules), cfg.Policy.String())
}

func LoadHttpProxyRules(cfgdir string) error {
	cfgfile := filepath.Join(cfgdir, aclConfigFile)
	if _, err := os.Stat(cfgfile); err != nil {
		return err
	}
	logger.Infof("Load proxy rules from %s", cfgfile)

	f, err := os.Open(cfgfile)
	if err != nil {
		return err
	}
	defer f.Close()

	cfg, err := decodeHttpProxyRules(f)
	if err != nil {
		return err
	}

	// The configuration is only updated if the configuration file
	// is correctly parsed.
	SetHttpProxyConfig(cfg.HttpProxy)

	return nil
}

func decodeHttpProxyRules(r io.Reader) (ACLConfig, error) {
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
		cfg, err := decodeHttpProxyRules(r)
		if err != nil {
			return err
		}
		if !dryRun {
			SetHttpProxyConfig(cfg.HttpProxy)

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
	defer f.Close()

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
			Urls:         v.Urls,
			Dists:        v.Dists,
			Components:   v.Components,
			PublicKey:    v.PublicKey,
			BaseUrlAlias: v.BaseUrlAlias,
		}
	}

	cfg.Git.Urls = make([]glob.Glob, len(globalInspectorsConfig.Git.Urls))
	copy(cfg.Git.Urls, globalInspectorsConfig.Git.Urls)

	cfg.Crafts.Urls = make([]glob.Glob, len(globalInspectorsConfig.Crafts.Urls))
	copy(cfg.Crafts.Urls, globalInspectorsConfig.Crafts.Urls)

	cfg.Store.Urls = make([]glob.Glob, len(globalInspectorsConfig.Store.Urls))
	copy(cfg.Store.Urls, globalInspectorsConfig.Store.Urls)

	cfg.BldBin.Urls = make([]glob.Glob, len(globalInspectorsConfig.BldBin.Urls))
	copy(cfg.BldBin.Urls, globalInspectorsConfig.BldBin.Urls)

	cfg.Chisel.Urls = make([]glob.Glob, len(globalInspectorsConfig.Chisel.Urls))
	copy(cfg.Chisel.Urls, globalInspectorsConfig.Chisel.Urls)

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
