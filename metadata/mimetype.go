// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023 Canonical Ltd.
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

package metadata

import (
	"archive/zip"
	"bytes"
	"regexp"

	"github.com/gabriel-vasile/mimetype"
)

func init() {
	mimetype.SetLimit(1 << 30) // input data is mmapped
	mimetype.Lookup("application/zip").Extend(whlDetector, "application/x-python-wheel", ".whl")
	mimetype.Lookup("text/plain").Extend(aptLegacyReleaseDetector, "application/x-apt-legacy-release", "")
	mimetype.Lookup("text/plain").Extend(aptReleaseDetector, "application/x-apt-release", "")
	mimetype.Lookup("application/x-xz").Extend(aptPackagesDetector, "application/x-apt-packages", "")
}

// zipMatches returns true if the zip file headers from in matches
// any of the path patterns. This is typically used in mime type
// detection of zipped files, such as Python wheels.
func zipMatches(in []byte, patterns ...string) bool {
	z, err := zip.NewReader(bytes.NewReader(in), int64(len(in)))
	if err != nil {
		return false
	}

	num := len(patterns)
	m := make(map[string]struct{}, num)

	for _, f := range z.File {
		for _, p := range patterns {
			if matched, _ := regexp.MatchString(p, f.Name); matched {
				m[p] = struct{}{}
				if len(m) == num {
					// all patterns found
					return true
				}
			}
		}
	}

	return false
}
