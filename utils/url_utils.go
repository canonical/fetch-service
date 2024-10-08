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

package utils

import (
	"fmt"
	"net/url"
)

// NormalizedOrigin returns the origin with the default HTTPS port number
// if it's not specified.
func NormalizedOrigin(u *url.URL) string {
	var origin string
	if u.Scheme == "https" && u.Port() == "" {
		origin = fmt.Sprintf("%s://%s:443", u.Scheme, u.Host)
	} else {
		origin = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}
	return origin
}
