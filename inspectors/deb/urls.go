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

package deb

import (
	"fmt"
	"net/url"
	"regexp"
)

// Recognized URL formats:
// -----------------------
// http://archive.ubuntu.com/ubuntu/pool/main/g/gcc-12/libgcc-s1_12.3.0-1ubuntu1%7e22.04_amd64.deb

var (
	validOrigins = []*regexp.Regexp{
		regexp.MustCompile(`^http://archive\.ubuntu\.com$`),
		regexp.MustCompile(`^http://[^./]\.archive\.ubuntu\.com$`),
		regexp.MustCompile(`^http://security\.ubuntu\.com$`),
		regexp.MustCompile(`^https://esm\.ubuntu\.com:443$`),
		regexp.MustCompile(`^http://ftpmaster.internal$`),
	}

	reDebPackage = regexp.MustCompile(`^/ubuntu/pool/([\w-]+)/.*/([^/_]+)_([^/_]+)_([^/_]+)\.deb$`)
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

type debPackageUrlInfo struct {
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
		repository:   fmt.Sprintf("%s://%s/ubuntu", u.Scheme, u.Host),
		component:    m[1],
		name:         m[2],
		version:      m[3],
		architecture: m[4],
	}
	return info, nil
}
