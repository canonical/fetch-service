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

package pip

import (
	"fmt"
	"net/url"
	"regexp"
)

// Recognized URL formats:
// -----------------------
// https://files.pythonhosted.org:443/packages/0a/[0-9a-f]{2}/[0-9a-f]{60}/\w+-[a-zA-Z0-9\.-]+\.whl

var (
	validOrigins = []*regexp.Regexp{
		regexp.MustCompile(`^https://files\.pythonhosted\.org:443$`),
	}

	// FIXME: using PyPI URLs as placeholders
	reWheel    = regexp.MustCompile(`^/packages/[0-9a-f]{2}/[0-9a-f]{2}/[0-9a-f]{60}/.+-[_a-zA-Z0-9\.-]+\.whl$`)
	reSdist    = regexp.MustCompile(`^/packages/[0-9a-f]{2}/[0-9a-f]{2}/[0-9a-f]{60}/.+-[_a-zA-Z0-9\.-]+\.tar\.gz$`)
	reMetadata = regexp.MustCompile(`^/packages/[0-9a-f]{2}/[0-9a-f]{2}/[0-9a-f]{60}/.+-[_a-zA-Z0-9\.-]+\.metadata$`)
)

func checkValidOrigin(u *url.URL) error {
	origin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	for _, h := range validOrigins {
		if !h.MatchString(origin) {
			return fmt.Errorf("invalid origin %s", origin)
		}
	}
	return nil
}

func checkWheelUrl(u *url.URL) error {
	if err := checkValidOrigin(u); err != nil {
		return err
	}
	if !reWheel.MatchString(u.Path) {
		return fmt.Errorf("invalid URL path %s", u.Path)
	}
	return nil
}

func checkSdistUrl(u *url.URL) error {
	if err := checkValidOrigin(u); err != nil {
		return err
	}
	if !reSdist.MatchString(u.Path) {
		return fmt.Errorf("invalid URL path %s", u.Path)
	}
	return nil
}

func checkMetadataUrl(u *url.URL) error {
	if err := checkValidOrigin(u); err != nil {
		return err
	}
	if !reMetadata.MatchString(u.Path) {
		return fmt.Errorf("invalid URL path %s", u.Path)
	}
	return nil
}
