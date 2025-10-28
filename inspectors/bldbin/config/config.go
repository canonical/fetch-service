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
	"fmt"
	"net/url"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/utils"
)

func checkRequestURL(cfg *BldBinInspectorConfig, u *url.URL, slog logger.Logger) error {
	requestURL := utils.NormalizedOrigin(u) + u.Path

	for _, h := range cfg.URLs {
		if h.Match(requestURL) {
			slog.Debugf("url matches %v\n", h)
			return nil
		}
	}
	return fmt.Errorf("invalid url %s", requestURL)
}

type BldBinInspectorConfig struct {
	URLs []glob.Glob `yaml:"urls"` // List of allowed URL glob patterns
}

type BldBinURLInfo struct {
}

func NewBldBinURLInfo(u *url.URL, cfg *BldBinInspectorConfig, slog logger.Logger) (*BldBinURLInfo, error) {
	if err := checkRequestURL(cfg, u, slog); err != nil {
		return nil, err
	}

	info := &BldBinURLInfo{}

	return info, nil
}
