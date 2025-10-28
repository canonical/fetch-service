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
	reStoreInfoAPI       = regexp.MustCompile(`^/v2/([a-z]+)/info/([a-zA-Z0-9-]+)$`)
	reStoreResolveAPI    = regexp.MustCompile(`^/v2/revisions/resolve$`)
	reStoreTransformsAPI = regexp.MustCompile(`^/v1/craft/workspaces/([a-zA-Z0-9-]+)/transforms$`)
	reStoreAppMedia      = regexp.MustCompile(`^/site_media/appmedia/([0-9]+)/([0-9]+)/([a-zA-Z0-9.-]+)$`)
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

type StoreInfoAPIUrlInfo struct {
	PackageType string
	PackageName string
}

func NewStoreInfoAPIUrlInfo(u *url.URL, cfg *StoreInspectorConfig, slog logger.Logger) (*StoreInfoAPIUrlInfo, error) {
	if err := checkRequestUrl(cfg, u, slog); err != nil {
		return nil, err
	}

	m := reStoreInfoAPI.FindStringSubmatch(u.Path)
	if len(m) != 3 {
		return nil, errors.New("not a valid store info API path")
	}
	info := &StoreInfoAPIUrlInfo{
		PackageType: m[1],
		PackageName: m[2],
	}

	return info, nil
}

type StoreResolveAPIUrlInfo struct {
}

func NewStoreResolveAPIUrlInfo(u *url.URL, cfg *StoreInspectorConfig, slog logger.Logger) (*StoreResolveAPIUrlInfo, error) {
	if err := checkRequestUrl(cfg, u, slog); err != nil {
		return nil, err
	}

	if !reStoreResolveAPI.MatchString(u.Path) {
		return nil, errors.New("not a valid store resolve_revisions API path")

	}

	return &StoreResolveAPIUrlInfo{}, nil
}

type StoreTransformsAPIUrlInfo struct {
	WorkspaceID string
}

func NewStoreTransformsAPIUrlInfo(u *url.URL, cfg *StoreInspectorConfig, slog logger.Logger) (*StoreTransformsAPIUrlInfo, error) {
	if err := checkRequestUrl(cfg, u, slog); err != nil {
		return nil, err
	}

	m := reStoreTransformsAPI.FindStringSubmatch(u.Path)
	if len(m) != 2 {
		return nil, errors.New("not a valid store transforms API path")

	}
	info := &StoreTransformsAPIUrlInfo{
		WorkspaceID: m[1],
	}

	return info, nil
}

type StoreAppMediaUrlInfo struct {
	Filename string
}

func NewStoreAppMediaUrlInfo(u *url.URL, cfg *StoreInspectorConfig, slog logger.Logger) (*StoreAppMediaUrlInfo, error) {
	if err := checkRequestUrl(cfg, u, slog); err != nil {
		return nil, err
	}

	m := reStoreAppMedia.FindStringSubmatch(u.Path)
	if len(m) != 4 {
		return nil, errors.New("not a valid store appmedia API path")

	}
	info := &StoreAppMediaUrlInfo{
		Filename: m[3],
	}

	return info, nil
}
