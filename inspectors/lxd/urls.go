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
// https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/index.json
// https://cloud-images.ubuntu.com:443/buildd/daily/streams/v1/com.ubuntu.cloud:daily:download.json
// http://cloud-images.ubuntu.com/buildd/daily/noble/20250629/noble-server-cloudimg-amd64-lxd_combined.tar.gz
// https://cloud-images.ubuntu.com:443/buildd/daily/noble/20250629/noble-server-cloudimg-amd64-lxd_combined.tar.gz
// https://images.lxd.canonical.com:443/meta/instance-types/all.yaml

var (
	validOrigins = []*regexp.Regexp{
		regexp.MustCompile(`^http://cloud-images.ubuntu.com$`),
		regexp.MustCompile(`^https://cloud-images.ubuntu.com:443$`),
		regexp.MustCompile(`^https://images.lxd.canonical.com:443$`),
	}

	reSimpleStreamsIndex    = regexp.MustCompile(`^/([\w-\/]+)/streams/v1/index.json$`)
	reSimpleStreamsDownload = regexp.MustCompile(`^/([\w-\/]+)/streams/v1/([\w-\.\/:]+):download.json$`)
	reProductItem           = regexp.MustCompile(`^/buildd/(daily|releases)/([\w-]+)/([\w-]+)/([\w+\.-]+\.tar.gz)$`)
	reInstanceTypes         = regexp.MustCompile(`^/meta/instance-types/all.yaml$`)
)

// Simple streams index

type SimpleStreamsIndexURLInfo struct {
	Stream string // The stream portion of the URL
}

func NewSimpleStreamsIndexURLInfo(u *url.URL) (*SimpleStreamsIndexURLInfo, error) {
	if err := checkValidOrigin(u); err != nil {
		return nil, err
	}

	m := reSimpleStreamsIndex.FindStringSubmatch(u.Path)
	if len(m) != 2 {
		return nil, fmt.Errorf("invalid URL path: %s", u.Path)
	}

	info := &SimpleStreamsIndexURLInfo{
		Stream: m[1],
	}
	return info, nil
}

// Simple stream product download request

type SimpleStreamsDownloadURLInfo struct {
	Stream   string // The stream portion of the URL
	ItemPath string // The image to download
}

func NewSimpleStreamsDownloadURLInfo(u *url.URL) (*SimpleStreamsDownloadURLInfo, error) {
	if err := checkValidOrigin(u); err != nil {
		return nil, err
	}

	m := reSimpleStreamsDownload.FindStringSubmatch(u.Path)
	if len(m) != 3 {
		return nil, fmt.Errorf("invalid URL path: %s", u.Path)
	}

	info := &SimpleStreamsDownloadURLInfo{
		Stream:   m[1],
		ItemPath: m[2],
	}
	return info, nil
}

// LXD product tarball

type ProductItemURLInfo struct {
	ImageType string // Daily or releases
	Series    string // The image series
	Date      string // The date of the daily image
	Name      string // The rootfs filename
}

func NewProductItemURLInfo(u *url.URL) (*ProductItemURLInfo, error) {
	if err := checkValidOrigin(u); err != nil {
		return nil, err
	}

	m := reProductItem.FindStringSubmatch(u.Path)
	if len(m) != 5 {
		return nil, fmt.Errorf("invalid URL path: %s", u.Path)
	}

	info := &ProductItemURLInfo{
		ImageType: m[1],
		Series:    m[2],
		Date:      m[3],
		Name:      m[4],
	}
	return info, nil
}

// LXD instance types

type InstanceTypesURLInfo struct{}

func newInstanceTypesURLInfo(u *url.URL) (*InstanceTypesURLInfo, error) {
	if err := checkValidOrigin(u); err != nil {
		return nil, err
	}

	if !reInstanceTypes.MatchString(u.Path) {
		return nil, fmt.Errorf("not a valid URL for LXD instance types")
	}

	return &InstanceTypesURLInfo{}, nil
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
