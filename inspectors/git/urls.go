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

package git

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Recognized URL formats:
// -----------------------
// https://github.com:443/user/project.git/info/refs?service=git-upload-pack
// https://github.com:443/user/project.git/git-upload-pack
// https://git.launchpad.net:443/project/git-upload-pack
// https://git.launchpad.net:443/~user/project/+git/project/

var (
	// FIXME: using github URL for now
	validOrigins = []*regexp.Regexp{
		regexp.MustCompile(`^https://github\.com:443$`),
		regexp.MustCompile(`^https://gopkg\.in:443$`),
		regexp.MustCompile(`^https://go\.googlesource\.com:443$`),
		regexp.MustCompile(`^https://git\.launchpad\.net:443$`),
	}

	reSmartQuery = regexp.MustCompile(`^/.*/info/refs$`)
	reUploadPack = regexp.MustCompile(`^/(.+/)*([^/]+)/git-upload-pack$`)
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

type smartQueryUrlInfo struct {
	service string
}

func newSmartQueryUrlInfo(u *url.URL) (*smartQueryUrlInfo, error) {
	if err := checkValidOrigin(u); err != nil {
		return nil, err
	}

	if !reSmartQuery.MatchString(u.Path) {
		return nil, fmt.Errorf("%s: not a valid git smart request URL path", u.Path)
	}

	q := u.Query()
	if val := q.Get("service"); val != "git-upload-pack" {
		return nil, fmt.Errorf("invalid service query %q", val)
	}

	info := &smartQueryUrlInfo{
		service: q.Get("service"),
	}
	return info, nil
}

type uploadPackUrlInfo struct {
	project string
}

func newUploadPackUrlInfo(u *url.URL) (*uploadPackUrlInfo, error) {
	if err := checkValidOrigin(u); err != nil {
		return nil, err
	}

	m := reUploadPack.FindStringSubmatch(u.Path)
	if len(m) != 3 && len(m) != 2 {
		return nil, fmt.Errorf("%s: not a valid git upload-pack URL path", u.Path)
	}
	info := &uploadPackUrlInfo{
		project: m[len(m)-1],
	}
	info.project, _ = strings.CutSuffix(info.project, ".git")

	return info, nil
}
