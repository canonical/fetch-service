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

func checkRequestURL(cfg *StoreInspectorConfig, u *url.URL, sl logger.Logger) error {
	requestURL := utils.NormalizedOrigin(u) + u.Path

	for _, h := range cfg.URLs {
		if h.Match(requestURL) {
			sl.Debugf("url matches %v", h)
			return nil
		}
	}
	return fmt.Errorf("invalid url %s", requestURL)
}

type StoreInspectorConfig struct {
	URLs []glob.Glob `yaml:"urls"` // List of allowed URL glob patterns
}

type StoreInfoAPIURLInfo struct {
	PackageType string
	PackageName string
}

func NewStoreInfoAPIURLInfo(u *url.URL, cfg *StoreInspectorConfig, sl logger.Logger) (*StoreInfoAPIURLInfo, error) {
	if err := checkRequestURL(cfg, u, sl); err != nil {
		return nil, err
	}

	m := reStoreInfoAPI.FindStringSubmatch(u.Path)
	if len(m) != 3 {
		return nil, errors.New("not a valid store info API path")
	}
	info := &StoreInfoAPIURLInfo{
		PackageType: m[1],
		PackageName: m[2],
	}

	return info, nil
}

type StoreResolveAPIURLInfo struct {
}

func NewStoreResolveAPIURLInfo(u *url.URL, cfg *StoreInspectorConfig, sl logger.Logger) (*StoreResolveAPIURLInfo, error) {
	if err := checkRequestURL(cfg, u, sl); err != nil {
		return nil, err
	}

	if !reStoreResolveAPI.MatchString(u.Path) {
		return nil, errors.New("not a valid store resolve_revisions API path")

	}

	return &StoreResolveAPIURLInfo{}, nil
}

type StoreTransformsAPIURLInfo struct {
	WorkspaceID string
}

func NewStoreTransformsAPIURLInfo(u *url.URL, cfg *StoreInspectorConfig, sl logger.Logger) (*StoreTransformsAPIURLInfo, error) {
	if err := checkRequestURL(cfg, u, sl); err != nil {
		return nil, err
	}

	m := reStoreTransformsAPI.FindStringSubmatch(u.Path)
	if len(m) != 2 {
		return nil, errors.New("not a valid store transforms API path")

	}
	info := &StoreTransformsAPIURLInfo{
		WorkspaceID: m[1],
	}

	return info, nil
}

type StoreAppMediaURLInfo struct {
	Filename string
}

func NewStoreAppMediaURLInfo(u *url.URL, cfg *StoreInspectorConfig, sl logger.Logger) (*StoreAppMediaURLInfo, error) {
	if err := checkRequestURL(cfg, u, sl); err != nil {
		return nil, err
	}

	m := reStoreAppMedia.FindStringSubmatch(u.Path)
	if len(m) != 4 {
		return nil, errors.New("not a valid store appmedia API path")

	}
	info := &StoreAppMediaURLInfo{
		Filename: m[3],
	}

	return info, nil
}
