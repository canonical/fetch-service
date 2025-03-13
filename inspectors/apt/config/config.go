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
	Urls       []glob.Glob `yaml:"urls"`       // List of allowed URL glob patterns
	Dists      []glob.Glob `yaml:"dists"`      // List of allowed dist glob patterns
	Components []glob.Glob `yaml:"components"` // List of allowed component glob patterns
	PublicKey  string      `yaml:"public-key"` // Repository public key
}

type AptInspectorConfig struct {
	Repositories map[string]AptInspectorConfigRepository
}

func checkRepositoryAndDist(cfg *AptInspectorConfig, u *url.URL) (string, string, string, error) {
	origin := utils.NormalizedOrigin(u)
	parts := strings.Split(u.Path, "/")

	pos := slices.Index(parts, "dists")
	if pos < 0 || len(parts) <= pos+2 {
		return "", "", "", fmt.Errorf("invalid repository URL: %s", u.String())
	}
	dist := parts[pos+1]

	repo := origin + strings.Join(parts[:pos], "/")
	repoCfgName, ok := repositoryIsAllowed(cfg, repo)
	if !ok {
		return "", "", "", fmt.Errorf("invalid repository: %s", repo)
	}

	if ok := distIsAllowed(cfg, repoCfgName, dist); !ok {
		return "", "", "", fmt.Errorf("invalid dist: %s", dist)
	}

	return repoCfgName, repo, dist, nil
}

func checkComponent(cfg *AptInspectorConfig, name string, u *url.URL) (string, error) {
	parts := strings.Split(u.Path, "/")

	pos := slices.Index(parts, "dists")
	if pos < 0 || len(parts) <= pos+3 {
		return "", fmt.Errorf("invalid repository URL: %s", u.String())
	}
	component := parts[pos+2]

	if !componentIsAllowed(cfg, name, component) {
		return "", fmt.Errorf("invalid component: %s", component)
	}

	return component, nil
}

// repositoryIsAllowed verifies if the given repository matches an allowed pattern.
func repositoryIsAllowed(cfg *AptInspectorConfig, repo string) (string, bool) {
	logger.Debugf("apt inspector config: check if repository '%s' is allowed", repo)
	for name, r := range cfg.Repositories {
		logger.Debugf("apt inspector config: check repository entry '%s'", name)
		for _, pattern := range r.Urls {
			if pattern.G.Match(repo) {
				logger.Debugf("apt inspector config: found repository '%s'", repo)
				return name, true
			}
		}
	}
	return "", false
}

// distIsAllowed verifies if the given dist matches an allowed pattern.
func distIsAllowed(cfg *AptInspectorConfig, name, dist string) bool {
	r := cfg.Repositories[name]
	logger.Debugf("apt inspector config: check if dist '%s' is allowed", dist)
	for _, pattern := range r.Dists {
		if pattern.G.Match(dist) {
			logger.Debugf("apt inspector config: found dist '%s'", dist)
			return true
		}
	}
	return false
}

// componentIsAllowed verifies if the given component matches an allowed pattern.
func componentIsAllowed(cfg *AptInspectorConfig, name, component string) bool {
	r := cfg.Repositories[name]
	logger.Debugf("apt inspector config: check if component '%s' is allowed", component)
	for _, pattern := range r.Components {
		if pattern.G.Match(component) {
			logger.Debugf("apt inspector config: found component '%s'", component)
			return true
		}
	}
	return false
}

type InReleaseUrlInfo struct {
	CfgName    string // Configuration entry name
	Origin     string // HTTP scheme and host
	Repository string // Apt repository root
	Dist       string // Repository dist name
}

func NewInReleaseUrlInfo(u *url.URL, cfg *AptInspectorConfig) (*InReleaseUrlInfo, error) {
	name, repo, dist, err := checkRepositoryAndDist(cfg, u)
	if err != nil {
		return nil, err
	}

	reInRelease := regexp.MustCompile(`/[\w-]+/dists/([\w-]+)/InRelease$`)
	if !reInRelease.MatchString(u.Path) {
		return nil, fmt.Errorf("invalid InRelease URL path: %s", u.Path)
	}

	info := &InReleaseUrlInfo{
		CfgName:    name,
		Origin:     utils.NormalizedOrigin(u),
		Repository: repo,
		Dist:       dist,
	}
	return info, nil
}

type PackagesUrlInfo struct {
	CfgName      string // Configuration entry name
	Origin       string // HTTP scheme and host
	Repository   string // Apt repository root
	Dist         string // Repository dist name
	Component    string // Repository component
	Architecture string //
	Digest       string // Digest from by-hash URL
}

func NewPackagesUrlInfo(u *url.URL, cfg *AptInspectorConfig) (*PackagesUrlInfo, error) {
	name, repo, dist, err := checkRepositoryAndDist(cfg, u)
	if err != nil {
		return nil, err
	}
	logger.Debugf("packages file cfgname=%s, repo=%s, dist=%s", name, repo, dist)

	component, err := checkComponent(cfg, name, u)
	if err != nil {
		return nil, err
	}
	logger.Debugf("packages file component=%s", component)

	rePackages := regexp.MustCompile(`/[\w-]+/dists/[\w-]+/[\w-]+/binary-(\w+)/by-hash/SHA256/([0-9a-f]{64})$`)
	m := rePackages.FindStringSubmatch(u.Path)
	if len(m) == 3 {
		info := &PackagesUrlInfo{
			CfgName:      name,
			Origin:       utils.NormalizedOrigin(u),
			Repository:   repo,
			Dist:         dist,
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
		info := &PackagesUrlInfo{
			CfgName:      name,
			Origin:       utils.NormalizedOrigin(u),
			Repository:   repo,
			Dist:         dist,
			Component:    component,
			Architecture: m[1],
		}
		return info, nil
	}

	return nil, fmt.Errorf("invalid Packages URL path: %s", u.Path)
}

type TranslationUrlInfo struct {
	CfgName    string // Configuration entry name
	Origin     string // HTTP scheme and host
	Repository string // Apt repository root
	Dist       string // Repository dist name
	Component  string // Repository component
	Digest     string // Digest from by-hash URL
}

func NewTranslationUrlInfo(u *url.URL, cfg *AptInspectorConfig) (*TranslationUrlInfo, error) {
	name, repo, dist, err := checkRepositoryAndDist(cfg, u)
	if err != nil {
		return nil, err
	}

	component, err := checkComponent(cfg, name, u)
	if err != nil {
		return nil, err
	}

	reTranslation := regexp.MustCompile(`/[\w-]+/dists/[\w-]+/[\w-]+/i18n/by-hash/SHA256/([0-9a-f]{64})$`)
	m := reTranslation.FindStringSubmatch(u.Path)
	if len(m) != 2 {
		return nil, fmt.Errorf("invalid translation URL path: %s", u.Path)
	}
	info := &TranslationUrlInfo{
		CfgName:    name,
		Origin:     utils.NormalizedOrigin(u),
		Repository: repo,
		Dist:       dist,
		Component:  component,
		Digest:     m[1],
	}
	return info, nil
}

type DebPackageUrlInfo struct {
	CfgName      string // Configuration entry name
	Origin       string // HTTP scheme and host
	Repository   string // Apt repository root
	Component    string // Repository component
	Name         string // Package name
	Version      string // Package version
	Architecture string // Package architecture
}

func NewDebPackageUrlInfo(u *url.URL, cfg *AptInspectorConfig) (*DebPackageUrlInfo, error) {
	origin := utils.NormalizedOrigin(u)
	parts := strings.Split(u.Path, "/")

	pos := slices.Index(parts, "pool")
	if pos < 0 || len(parts) <= pos+2 {
		return nil, fmt.Errorf("invalid repository URL: %s", u.String())
	}

	repo := origin + strings.Join(parts[:pos], "/")
	repoCfgName, ok := repositoryIsAllowed(cfg, repo)
	if !ok {
		return nil, fmt.Errorf("invalid repository: %s", repo)
	}

	reDebPackage := regexp.MustCompile(`/[\w-]+/pool/([\w-]+)/.*/([^/_]+)_([^/_]+)_([^/_]+)\.deb$`)
	m := reDebPackage.FindStringSubmatch(u.Path)
	if len(m) != 5 {
		return nil, fmt.Errorf("%s: not a valid deb package URL path", u.Path)
	}
	info := &DebPackageUrlInfo{
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
