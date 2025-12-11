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
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/utils"
)

type AptInspectorConfigRepository struct {
	URLs         []glob.Glob `yaml:"urls"`           // List of allowed URL glob patterns
	Suites       []glob.Glob `yaml:"suites"`         // List of allowed dist glob patterns
	Components   []glob.Glob `yaml:"components"`     // List of allowed component glob patterns
	PublicKey    string      `yaml:"public-key"`     // Repository public key
	BaseURLAlias string      `yaml:"base-url-alias"` // Alias for URL scheme and hostname
}

type AptInspectorConfig struct {
	Repositories map[string]AptInspectorConfigRepository
}

func checkRepositoryAndSuite(cfg *AptInspectorConfig, u *url.URL, slog logger.Logger) (string, string, string, error) {
	origin := utils.NormalizedOrigin(u)
	parts := strings.Split(u.Path, "/")

	pos := slices.Index(parts, "dists")
	if pos < 0 || len(parts) <= pos+2 {
		return "", "", "", fmt.Errorf("invalid repository URL: %s", u.String())
	}
	suite := parts[pos+1]

	repo := origin + strings.Join(parts[:pos], "/")
	repoCfgName, ok := repositoryIsAllowed(cfg, repo, slog)
	if !ok {
		return "", "", "", fmt.Errorf("invalid repository: %s", repo)
	}

	if ok := suiteIsAllowed(cfg, repoCfgName, suite, slog); !ok {
		return "", "", "", fmt.Errorf("invalid series: %s", suite)
	}

	return repoCfgName, repo, suite, nil
}

func checkComponent(cfg *AptInspectorConfig, name string, u *url.URL, slog logger.Logger) (string, error) {
	parts := strings.Split(u.Path, "/")

	pos := slices.Index(parts, "dists")
	if pos < 0 || len(parts) <= pos+3 {
		return "", fmt.Errorf("invalid repository URL: %s", u.String())
	}
	component := parts[pos+2]

	if !componentIsAllowed(cfg, name, component, slog) {
		return "", fmt.Errorf("invalid component: %s", component)
	}

	return component, nil
}

// repositoryIsAllowed verifies if the given repository matches an allowed pattern.
func repositoryIsAllowed(cfg *AptInspectorConfig, repo string, slog logger.Logger) (string, bool) {
	slog.Debugf("apt inspector config: check if repository '%s' is allowed", repo)
	for name, r := range cfg.Repositories {
		slog.Debugf("apt inspector config: check repository entry '%s'", name)
		for _, pattern := range r.URLs {
			if pattern.G.Match(repo) {
				slog.Debugf("apt inspector config: found repository '%s'", repo)
				return name, true
			}
		}
	}
	return "", false
}

// suiteIsAllowed verifies if the given suite matches an allowed pattern.
func suiteIsAllowed(cfg *AptInspectorConfig, name, suite string, slog logger.Logger) bool {
	r := cfg.Repositories[name]
	slog.Debugf("apt inspector config: check if suite '%s' is allowed", suite)
	for _, pattern := range r.Suites {
		if pattern.G.Match(suite) {
			slog.Debugf("apt inspector config: found suite '%s'", suite)
			return true
		}
	}
	return false
}

// componentIsAllowed verifies if the given component matches an allowed pattern.
func componentIsAllowed(cfg *AptInspectorConfig, name, component string, slog logger.Logger) bool {
	r := cfg.Repositories[name]
	slog.Debugf("apt inspector config: check if component '%s' is allowed", component)
	for _, pattern := range r.Components {
		if pattern.G.Match(component) {
			slog.Debugf("apt inspector config: found component '%s'", component)
			return true
		}
	}
	return false
}

type InReleaseURLInfo struct {
	CfgName    string // Configuration entry name
	Origin     string // HTTP scheme and host
	Repository string // Apt repository root
	Suite      string // Repository suite (<series>-<pocket>)
}

func NewInReleaseURLInfo(u *url.URL, cfg *AptInspectorConfig, slog logger.Logger) (*InReleaseURLInfo, error) {
	name, repo, suite, err := checkRepositoryAndSuite(cfg, u, slog)
	if err != nil {
		return nil, err
	}

	reInRelease := regexp.MustCompile(`/[\w-]+/dists/([\w-]+)/InRelease$`)
	if !reInRelease.MatchString(u.Path) {
		return nil, fmt.Errorf("invalid InRelease URL path: %s", u.Path)
	}

	info := &InReleaseURLInfo{
		CfgName:    name,
		Origin:     utils.NormalizedOrigin(u),
		Repository: repo,
		Suite:      suite,
	}
	return info, nil
}

type PackagesURLInfo struct {
	CfgName      string // Configuration entry name
	Origin       string // HTTP scheme and host
	Repository   string // Apt repository root
	Suite        string // Repository suite name (<series>-<pocket>)
	Component    string // Repository component
	Architecture string //
	Digest       string // Digest from by-hash URL
}

func NewPackagesURLInfo(u *url.URL, cfg *AptInspectorConfig, slog logger.Logger) (*PackagesURLInfo, error) {
	name, repo, suite, err := checkRepositoryAndSuite(cfg, u, slog)
	if err != nil {
		return nil, err
	}
	slog.Debugf("packages file cfgname=%s, repo=%s, suite=%s", name, repo, suite)

	component, err := checkComponent(cfg, name, u, slog)
	if err != nil {
		return nil, err
	}
	slog.Debugf("packages file component=%s", component)

	rePackages := regexp.MustCompile(`/[\w-]+/dists/[\w-]+/[\w-]+/binary-(\w+)/by-hash/SHA256/([0-9a-f]{64})$`)
	m := rePackages.FindStringSubmatch(u.Path)
	if len(m) == 3 {
		info := &PackagesURLInfo{
			CfgName:      name,
			Origin:       utils.NormalizedOrigin(u),
			Repository:   repo,
			Suite:        suite,
			Component:    component,
			Architecture: m[1],
			Digest:       m[2],
		}
		return info, nil
	}

	// Chisel fetches the Packages.gz file by name, e.g.:
	// GET https://esm.ubuntu.com:443/fips/ubuntu/dists/focal/main/binary-amd64/Packages.gz
	rePackages = regexp.MustCompile(`/[\w-]+/dists/[\w-]+/[\w-]+/binary-(\w+)/Packages.gz$`)
	m = rePackages.FindStringSubmatch(u.Path)
	if len(m) == 2 {
		info := &PackagesURLInfo{
			CfgName:      name,
			Origin:       utils.NormalizedOrigin(u),
			Repository:   repo,
			Suite:        suite,
			Component:    component,
			Architecture: m[1],
		}
		return info, nil
	}

	return nil, fmt.Errorf("invalid Packages URL path: %s", u.Path)
}

type TranslationURLInfo struct {
	CfgName    string // Configuration entry name
	Origin     string // HTTP scheme and host
	Repository string // Apt repository root
	Suite      string // Repository dist name
	Component  string // Repository component
	Digest     string // Digest from by-hash URL
}

func NewTranslationURLInfo(u *url.URL, cfg *AptInspectorConfig, slog logger.Logger) (*TranslationURLInfo, error) {
	name, repo, suite, err := checkRepositoryAndSuite(cfg, u, slog)
	if err != nil {
		return nil, err
	}

	component, err := checkComponent(cfg, name, u, slog)
	if err != nil {
		return nil, err
	}

	reTranslation := regexp.MustCompile(`/[\w-]+/dists/[\w-]+/[\w-]+/i18n/Translation-[\w-]+$`)
	digest := ""

	if !reTranslation.MatchString(u.Path) {
		reTranslationByHash := regexp.MustCompile(`/[\w-]+/dists/[\w-]+/[\w-]+/i18n/by-hash/SHA256/([0-9a-f]{64})$`)
		m := reTranslationByHash.FindStringSubmatch(u.Path)
		if len(m) != 2 {
			return nil, fmt.Errorf("invalid translation URL path: %s", u.Path)
		}
		digest = m[1]
	}

	info := &TranslationURLInfo{
		CfgName:    name,
		Origin:     utils.NormalizedOrigin(u),
		Repository: repo,
		Suite:      suite,
		Component:  component,
		Digest:     digest,
	}
	return info, nil
}

type CommandsURLInfo struct {
	CfgName    string // Configuration entry name
	Origin     string // HTTP scheme and host
	Repository string // Apt repository root
	Suite      string // Repository suite name
	Component  string // Repository component
	Digest     string // Digest from by-hash URL
}

func NewCommandURLInfo(u *url.URL, cfg *AptInspectorConfig, slog logger.Logger) (*CommandsURLInfo, error) {
	name, repo, suite, err := checkRepositoryAndSuite(cfg, u, slog)
	if err != nil {
		return nil, err
	}

	component, err := checkComponent(cfg, name, u, slog)
	if err != nil {
		return nil, err
	}

	reCommands := regexp.MustCompile(`/[\w-]+/dists/[\w-]+/[\w-]+/cnf/Commands-[\.\w-]+$`)
	digest := ""

	if !reCommands.MatchString(u.Path) {
		reCommandsByHash := regexp.MustCompile(`/[\w-]+/dists/[\w-]+/[\w-]+/cnf/by-hash/SHA256/([0-9a-f]{64})$`)
		m := reCommandsByHash.FindStringSubmatch(u.Path)
		if len(m) != 2 {
			return nil, fmt.Errorf("invalid commands URL path: %s", u.Path)
		}
		digest = m[1]
	}
	info := &CommandsURLInfo{
		CfgName:    name,
		Origin:     utils.NormalizedOrigin(u),
		Repository: repo,
		Suite:      suite,
		Component:  component,
		Digest:     digest,
	}
	return info, nil
}

type DebPackageURLInfo struct {
	CfgName      string // Configuration entry name
	Origin       string // HTTP scheme and host
	Repository   string // Apt repository root
	Component    string // Repository component
	Name         string // Package name
	Version      string // Package version
	Architecture string // Package architecture
}

func NewDebPackageURLInfo(u *url.URL, cfg *AptInspectorConfig, slog logger.Logger) (*DebPackageURLInfo, error) {
	origin := utils.NormalizedOrigin(u)
	parts := strings.Split(u.Path, "/")

	pos := slices.Index(parts, "pool")
	if pos < 0 || len(parts) <= pos+2 {
		return nil, fmt.Errorf("invalid repository URL: %s", u.String())
	}

	repo := origin + strings.Join(parts[:pos], "/")
	repoCfgName, ok := repositoryIsAllowed(cfg, repo, slog)
	if !ok {
		return nil, fmt.Errorf("invalid repository: %s", repo)
	}

	reDebPackage := regexp.MustCompile(`/[\w-]+/pool/([\w-]+)/.*/([^/_]+)_([^/_]+)_([^/_]+)\.deb$`)
	m := reDebPackage.FindStringSubmatch(u.Path)
	if len(m) != 5 {
		return nil, fmt.Errorf("%s: not a valid deb package URL path", u.Path)
	}
	info := &DebPackageURLInfo{
		CfgName:      repoCfgName,
		Origin:       origin,
		Repository:   repo,
		Component:    m[1],
		Name:         m[2],
		Version:      m[3],
		Architecture: m[4],
	}
	return info, nil
}
