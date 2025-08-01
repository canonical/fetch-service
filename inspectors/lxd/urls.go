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

package lxd

import (
	"fmt"
	"net/url"
	"regexp"
)

// Recognized URL format:
// ----------------------
// http://cloud-images.ubuntu.com/buildd/daily/noble/20250629/noble-server-cloudimg-amd64-lxd_combined.tar.gz
// https://cloud-images.ubuntu.com:443/buildd/daily/noble/20250629/noble-server-cloudimg-amd64-lxd_combined.tar.gz

var (
	validOrigins = []*regexp.Regexp{
		regexp.MustCompile(`^http://cloud-images.ubuntu.com$`),
		regexp.MustCompile(`^https://cloud-images.ubuntu.com:443$`),
	}

	reSimpleStreamsIndex = regexp.MustCompile(`^/([\w-\/]+)/streams/v1/index.json$`)
	reRootfs             = regexp.MustCompile(`^/buildd/daily/([\w-]+)/(\d+)/([\w+\.-]+\.tar.gz)$`)
)

type SimpleStreamsIndexUrlInfo struct {
	Stream string // The stream portion of the URL
}

func NewSimpleStreamsIndexUrlInfo(u *url.URL) (*SimpleStreamsIndexUrlInfo, error) {
	if err := checkValidOrigin(u); err != nil {
		return nil, err
	}

	m := reSimpleStreamsIndex.FindStringSubmatch(u.Path)
	if len(m) != 2 {
		return nil, fmt.Errorf("invalid URL path: %s", u.Path)
	}

	info := &SimpleStreamsIndexUrlInfo{
		Stream: m[1],
	}
	return info, nil
}

type RootfsUrlInfo struct {
	Series string // The image series
	Date   string // The date of the daily image
	Name   string // The rootfs filename
}

func NewRootfsUrlInfo(u *url.URL) (*RootfsUrlInfo, error) {
	if err := checkValidOrigin(u); err != nil {
		return nil, err
	}

	m := reRootfs.FindStringSubmatch(u.Path)
	if len(m) != 4 {
		return nil, fmt.Errorf("invalid URL path: %s", u.Path)
	}

	info := &RootfsUrlInfo{
		Series: m[1],
		Date:   m[2],
		Name:   m[3],
	}
	return info, nil
}

func checkValidOrigin(u *url.URL) error {
	origin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	for _, h := range validOrigins {
		if h.MatchString(origin) {
			return nil
		}
	}
	return fmt.Errorf("invalid origin %s", origin)
}
