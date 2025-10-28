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

package gomod

import (
	"fmt"
	"net/url"
	"regexp"
)

var (
	validOrigins  = []*regexp.Regexp{}
	reGoModuleGit = regexp.MustCompile(`^/([^/]+/)*([^/]+)/git-upload-pack$`)
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

type goModuleURLInfo struct {
	project string
}

func newGoModuleGitURLInfo(u *url.URL) (*goModuleURLInfo, error) {
	if err := checkValidOrigin(u); err != nil {
		return nil, err
	}

	m := reGoModuleGit.FindStringSubmatch(u.Path)
	if len(m) != 3 && len(m) != 2 {
		return nil, fmt.Errorf("%s: not a valid URL path for git go modules", u.Path)
	}
	info := &goModuleURLInfo{
		project: m[len(m)-1],
	}

	return info, nil
}
