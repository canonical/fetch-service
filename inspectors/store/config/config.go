// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2025 Canonical Ltd.
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

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/utils"
)

var (
	reStoreApi = regexp.MustCompile(`^/v2/([a-z]+)/info/([a-zA-Z0-9-]+)$`)
)

func checkRequestUrl(cfg *StoreInspectorConfig, u *url.URL, slog logger.Logger) error {
	requestUrl := utils.NormalizedOrigin(u) + u.Path

	for _, h := range cfg.Urls {
		if h.Match(requestUrl) {
			slog.Debugf("url matches %v\n", h)
			return nil
		}
	}
	return fmt.Errorf("invalid url %s", requestUrl)
}

type StoreInspectorConfig struct {
	Urls []glob.Glob `yaml:"urls"` // List of allowed URL glob patterns
}

type StoreApiUrlInfo struct {
	PackageType string
	PackageName string
}

func NewStoreApiUrlInfo(u *url.URL, cfg *StoreInspectorConfig, slog logger.Logger) (*StoreApiUrlInfo, error) {
	if err := checkRequestUrl(cfg, u, slog); err != nil {
		return nil, err
	}

	m := reStoreApi.FindStringSubmatch(u.Path)
	if len(m) != 3 {
		return nil, errors.New("not a valid store API path")
	}
	info := &StoreApiUrlInfo{
		PackageType: m[1],
		PackageName: m[2],
	}

	return info, nil
}
