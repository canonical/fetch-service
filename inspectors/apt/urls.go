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

package apt

import (
	"fmt"
	"net/url"
	"regexp"
)

// Recognized URL formats:
// -----------------------
// http://archive.ubuntu.com/ubuntu/dists/jammy/InRelease
// http://archive.ubuntu.com/ubuntu/dists/jammy/main/binary-amd64/by-hash/SHA256/05cd4debe8...
// http://archive.ubuntu.com/ubuntu/dists/jammy/i18n/by-hash/SHA256/58839e438...
// http://archive.ubuntu.com/ubuntu/pool/main/g/gcc-12/libgcc-s1_12.3.0-1ubuntu1%7e22.04_amd64.deb

var (
	validOrigins = []*regexp.Regexp{
		regexp.MustCompile(`^http://archive\.ubuntu\.com$`),
		regexp.MustCompile(`^http://[^./]\.archive\.ubuntu\.com$`),
		regexp.MustCompile(`^http://security\.ubuntu\.com$`),
		regexp.MustCompile(`^https://esm\.ubuntu\.com:443$`),
		regexp.MustCompile(`^http://ftpmaster.internal$`),
	}

	reInRelease   = regexp.MustCompile(`^/ubuntu/dists/([\w-]+)/InRelease$`)
	rePackages    = regexp.MustCompile(`^/ubuntu/dists/([\w-]+)/([\w-]+)/binary-(\w+)/by-hash/SHA256/([0-9a-f]{64})$`)
	reTranslation = regexp.MustCompile(`^/ubuntu/dists/([\w-]+)/([\w-]+)/i18n/by-hash/SHA256/([0-9a-f]{64})$`)
	reDebPackage  = regexp.MustCompile(`^/ubuntu/pool/([\w-]+)/.*/([^/_]+)_([^/_]+)_([^/_]+)\.deb$`)
)

func checkValidOrigin(u *url.URL) error {
	origin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	for _, h := range validOrigins {
		if h.MatchString(origin) {
			return nil
		}
	}
	return fmt.Errorf("invalid origin %s", origin)
}

type inReleaseUrlInfo struct {
	origin       string
	repository   string
	distribution string
}

func newInReleaseUrlInfo(u *url.URL) (*inReleaseUrlInfo, error) {
	if err := checkValidOrigin(u); err != nil {
		return nil, err
	}

	m := reInRelease.FindStringSubmatch(u.Path)
	if len(m) != 2 {
		return nil, fmt.Errorf("%s: not a valid InRelease URL path", u.Path)
	}
	info := &inReleaseUrlInfo{
		origin:       fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		repository:   fmt.Sprintf("%s://%s/ubuntu", u.Scheme, u.Host),
		distribution: m[1],
	}
	return info, nil
}

type packagesUrlInfo struct {
	origin       string
	repository   string
	distribution string
	component    string
	architecture string
	digest       string
}

func newPackagesUrlInfo(u *url.URL) (*packagesUrlInfo, error) {
	if err := checkValidOrigin(u); err != nil {
		return nil, err
	}

	m := rePackages.FindStringSubmatch(u.Path)
	if len(m) != 5 {
		return nil, fmt.Errorf("%s: not a valid Packages URL path", u.Path)
	}
	info := &packagesUrlInfo{
		origin:       fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		repository:   fmt.Sprintf("%s://%s/ubuntu", u.Scheme, u.Host),
		distribution: m[1],
		component:    m[2],
		architecture: m[3],
		digest:       m[4],
	}
	return info, nil
}

type translationUrlInfo struct {
	origin       string
	repository   string
	distribution string
	component    string
	digest       string
}

func newTranslationUrlInfo(u *url.URL) (*translationUrlInfo, error) {
	if err := checkValidOrigin(u); err != nil {
		return nil, err
	}

	m := reTranslation.FindStringSubmatch(u.Path)
	if len(m) != 4 {
		return nil, fmt.Errorf("%s: not a valid translation URL path", u.Path)
	}
	info := &translationUrlInfo{
		origin:       fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		repository:   fmt.Sprintf("%s://%s/ubuntu", u.Scheme, u.Host),
		distribution: m[1],
		component:    m[2],
		digest:       m[3],
	}
	return info, nil
}

type debPackageUrlInfo struct {
	origin       string
	repository   string
	component    string
	name         string
	version      string
	architecture string
}

func newDebPackageUrlInfo(u *url.URL) (*debPackageUrlInfo, error) {
	if err := checkValidOrigin(u); err != nil {
		return nil, err
	}

	m := reDebPackage.FindStringSubmatch(u.Path)
	if len(m) != 5 {
		return nil, fmt.Errorf("%s: not a valid deb package URL path", u.Path)
	}
	info := &debPackageUrlInfo{
		origin:       fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		repository:   fmt.Sprintf("%s://%s/ubuntu", u.Scheme, u.Host),
		component:    m[1],
		name:         m[2],
		version:      m[3],
		architecture: m[4],
	}
	return info, nil
}
