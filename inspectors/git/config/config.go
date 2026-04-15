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
	URLs []glob.Glob `yaml:"urls"` // List of allowed URL glob patterns
}

func checkRequestURL(cfg *GitInspectorConfig, u *url.URL, sl logger.Logger) error {
	reqURL := utils.NormalizedOrigin(u) + u.Path

	for _, h := range cfg.URLs {
		if h.Match(reqURL) {
			sl.Debugf("git url matches %v", h)
			return nil
		}
	}
	return fmt.Errorf("invalid url %s", reqURL)
}

type SmartQueryURLInfo struct {
	Service string
}

func NewSmartQueryURLInfo(u *url.URL, cfg *GitInspectorConfig, sl logger.Logger) (*SmartQueryURLInfo, error) {
	if err := checkRequestURL(cfg, u, sl); err != nil {
		return nil, err
	}

	if !reSmartQuery.MatchString(u.Path) {
		return nil, errors.New("not a valid git smart query path")
	}

	q := u.Query()
	if val := q.Get("service"); val != "git-upload-pack" {
		return nil, fmt.Errorf("invalid service query %q", val)
	}

	info := &SmartQueryURLInfo{
		Service: q.Get("service"),
	}
	return info, nil
}

type UploadPackURLInfo struct {
	Project string
}

func NewUploadPackURLInfo(u *url.URL, cfg *GitInspectorConfig, sl logger.Logger) (*UploadPackURLInfo, error) {
	if err := checkRequestURL(cfg, u, sl); err != nil {
		return nil, err
	}

	m := reUploadPack.FindStringSubmatch(u.Path)
	if len(m) != 3 && len(m) != 2 {
		return nil, errors.New("not a valid git upload-pack path")
	}
	info := &UploadPackURLInfo{
		Project: m[len(m)-1], // assuming the project name is encoded in the URL
	}
	info.Project, _ = strings.CutSuffix(info.Project, ".git")

	return info, nil
}
