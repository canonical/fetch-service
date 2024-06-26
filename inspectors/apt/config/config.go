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

package config

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/gobwas/glob"

	"github.com/canonical/fetch-service/logger"
)

type Glob struct {
	G glob.Glob
}

func (t *Glob) UnmarshalYAML(unmarshal func(v interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	g, err := glob.Compile(s)
	if err != nil {
		return err
	}

	*t = Glob{g}
	return nil
}

type AptInspectorConfigRepository struct {
	Urls      []Glob `yaml:"urls"`
	Dists     []Glob `yaml:"dists"`
	PublicKey string `yaml:"public-key"`
}

type AptInspectorConfig struct {
	Repositories map[string]AptInspectorConfigRepository
}

func checkRepositoryAndDist(cfg *AptInspectorConfig, u *url.URL) (string, string, string, error) {
	repo := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	parts := strings.Split(u.Path, "/")

	pos := slices.Index(parts, "dists")
	if pos < 0 || len(parts) <= pos+2 {
		return "", "", "", fmt.Errorf("invalid repository URL %s", u.String())
	}
	dist := parts[pos+1]

	repo += strings.Join(parts[:pos], "/")
	repoCfgName, ok := repositoryIsAllowed(cfg, repo)
	if !ok {
		return "", "", "", fmt.Errorf("invalid repository %s", repo)
	}

	if ok := distIsAllowed(cfg, repoCfgName, dist); !ok {
		return "", "", "", fmt.Errorf("invalid dist %s", dist)
	}

	return repoCfgName, repo, dist, nil
}

// repositoryIsAllowed verifies if the given repository matches an allowed pattern.
func repositoryIsAllowed(cfg *AptInspectorConfig, repo string) (string, bool) {
	logger.Debugf("apt inspector config: check repository '%s'", repo)
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
	logger.Debugf("apt inspector config: parsing repository '%s'", name)
	for _, pattern := range r.Dists {
		if pattern.G.Match(dist) {
			logger.Debugf("apt inspector config: found dist '%s'", dist)
			return true
		}
	}
	return false
}

type InReleaseUrlInfo struct {
	CfgName    string
	Origin     string
	Repository string
	Dist       string
}

func NewInReleaseUrlInfo(u *url.URL, cfg *AptInspectorConfig) (*InReleaseUrlInfo, error) {
	name, repo, dist, err := checkRepositoryAndDist(cfg, u)
	if err != nil {
		return nil, err
	}

	reInRelease := regexp.MustCompile(`^/[\w]+/dists/([\w-]+)/InRelease$`)
	if !reInRelease.MatchString(u.Path) {
		return nil, fmt.Errorf("URL path mismatch for InRelease file")
	}

	info := &InReleaseUrlInfo{
		CfgName:    name,
		Origin:     fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		Repository: repo,
		Dist:       dist,
	}
	return info, nil
}
