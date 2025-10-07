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
	reStoreInfoApi       = regexp.MustCompile(`^/v2/([a-z]+)/info/([a-zA-Z0-9-]+)$`)
	reStoreResolveApi    = regexp.MustCompile(`^/v2/revisions/resolve$`)
	reStoreTransformsApi = regexp.MustCompile(`^/v1/craft/workspaces/([a-zA-Z0-9-]+)/transforms$`)
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

type StoreInfoApiUrlInfo struct {
	PackageType string
	PackageName string
}

func NewStoreInfoApiUrlInfo(u *url.URL, cfg *StoreInspectorConfig, slog logger.Logger) (*StoreInfoApiUrlInfo, error) {
	if err := checkRequestUrl(cfg, u, slog); err != nil {
		return nil, err
	}

	m := reStoreInfoApi.FindStringSubmatch(u.Path)
	if len(m) != 3 {
		return nil, errors.New("not a valid store info API path")
	}
	info := &StoreInfoApiUrlInfo{
		PackageType: m[1],
		PackageName: m[2],
	}

	return info, nil
}

type StoreResolveApiUrlInfo struct {
}

func NewStoreResolveApiUrlInfo(u *url.URL, cfg *StoreInspectorConfig, slog logger.Logger) (*StoreResolveApiUrlInfo, error) {
	if err := checkRequestUrl(cfg, u, slog); err != nil {
		return nil, err
	}

	if !reStoreResolveApi.MatchString(u.Path) {
		return nil, errors.New("not a valid store resolve_revisions API path")

	}

	return &StoreResolveApiUrlInfo{}, nil
}

type StoreTransformsApiUrlInfo struct {
	WorkspaceID string
}

func NewStoreTransformsApiUrlInfo(u *url.URL, cfg *StoreInspectorConfig, slog logger.Logger) (*StoreTransformsApiUrlInfo, error) {
	if err := checkRequestUrl(cfg, u, slog); err != nil {
		return nil, err
	}

	m := reStoreTransformsApi.FindStringSubmatch(u.Path)
	if len(m) != 2 {
		return nil, errors.New("not a valid store transforms API path")

	}
	info := &StoreTransformsApiUrlInfo{
		WorkspaceID: m[1],
	}

	return info, nil
}
