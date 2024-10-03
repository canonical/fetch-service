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
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/utils"
)

var (
	reSmartQuery = regexp.MustCompile(`^/.*/info/refs$`)
	reUploadPack = regexp.MustCompile(`^/(.+/)*([^/]+)/git-upload-pack$`)
)

type GitInspectorConfig struct {
	Origins []glob.Glob `yaml:"origins"` // List of allowed URL glob patterns
}

func checkServerOrigin(cfg *GitInspectorConfig, u *url.URL) error {
	origin := utils.NormalizedOrigin(u)

	for _, h := range cfg.Origins {
		if h.Match(origin) {
			logger.Debugf("git url origin matches %v\n", h)
			return nil
		}
	}
	return fmt.Errorf("invalid origin %s", origin)
}

type SmartQueryUrlInfo struct {
	Service string
}

func NewSmartQueryUrlInfo(u *url.URL, cfg *GitInspectorConfig) (*SmartQueryUrlInfo, error) {
	if err := checkServerOrigin(cfg, u); err != nil {
		return nil, err
	}

	if !reSmartQuery.MatchString(u.Path) {
		return nil, errors.New("not a valid git smart query path")
	}

	q := u.Query()
	if val := q.Get("service"); val != "git-upload-pack" {
		return nil, fmt.Errorf("invalid service query %q", val)
	}

	info := &SmartQueryUrlInfo{
		Service: q.Get("service"),
	}
	return info, nil
}

type UploadPackUrlInfo struct {
	Project string
}

func NewUploadPackUrlInfo(u *url.URL, cfg *GitInspectorConfig) (*UploadPackUrlInfo, error) {
	if err := checkServerOrigin(cfg, u); err != nil {
		return nil, err
	}

	m := reUploadPack.FindStringSubmatch(u.Path)
	if len(m) != 3 && len(m) != 2 {
		return nil, errors.New("not a valid git upload-pack path")
	}
	info := &UploadPackUrlInfo{
		Project: m[len(m)-1], // assuming the project name is encoded in the URL
	}
	info.Project, _ = strings.CutSuffix(info.Project, ".git")

	return info, nil
}
