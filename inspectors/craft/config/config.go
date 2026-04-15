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
	reSourcecraft = regexp.MustCompile(`^/(.+/)*([^/]+)/git-upload-pack$`)
)

func checkRequestURL(cfg *CraftsInspectorConfig, u *url.URL, sl logger.Logger) error {
	requestURL := utils.NormalizedOrigin(u) + u.Path

	for _, h := range cfg.URLs {
		if h.Match(requestURL) {
			sl.Debugf("url matches %v", h)
			return nil
		}
	}
	return fmt.Errorf("invalid url %s", requestURL)
}

type CraftsInspectorConfig struct {
	URLs []glob.Glob `yaml:"urls"` // List of allowed URL glob patterns
}

type CraftURLInfo struct {
	Project string
}

func NewCraftURLInfo(u *url.URL, cfg *CraftsInspectorConfig, sl logger.Logger) (*CraftURLInfo, error) {
	if err := checkRequestURL(cfg, u, sl); err != nil {
		return nil, err
	}

	m := reSourcecraft.FindStringSubmatch(u.Path)
	if len(m) != 3 && len(m) != 2 {
		return nil, errors.New("not a valid *craft upload-pack path")
	}
	info := &CraftURLInfo{
		Project: m[len(m)-1], // assuming the project name is encoded in the URL
	}
	info.Project, _ = strings.CutSuffix(info.Project, ".git")

	return info, nil
}
