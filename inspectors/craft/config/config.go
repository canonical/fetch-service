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
	reSourcecraft = regexp.MustCompile(`^/(.+/)*([^/]+)/git-upload-pack$`)
)

func checkServerOrigin(origins []glob.Glob, u *url.URL) error {
	origin := utils.NormalizedOrigin(u)

	for _, h := range origins {
		if h.Match(origin) {
			logger.Debugf("url origin matches %v\n", h)
			return nil
		}
	}
	return fmt.Errorf("invalid origin %s", origin)
}

type SourcecraftInspectorConfig struct {
	Origins []glob.Glob `yaml:"origins"` // List of allowed URL origin glob patterns
}

type SourcecraftUrlInfo struct {
	Project string
}

func NewSourcecraftUrlInfo(u *url.URL, cfg *SourcecraftInspectorConfig) (*SourcecraftUrlInfo, error) {
	if err := checkServerOrigin(cfg.Origins, u); err != nil {
		return nil, err
	}

	m := reSourcecraft.FindStringSubmatch(u.Path)
	if len(m) != 3 && len(m) != 2 {
		return nil, errors.New("not a valid sourcecraft upload-pack path")
	}
	info := &SourcecraftUrlInfo{
		Project: m[len(m)-1], // assuming the project name is encoded in the URL
	}
	info.Project, _ = strings.CutSuffix(info.Project, ".git")

	return info, nil
}
