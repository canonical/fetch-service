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

package utils

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/canonical/fetch-service/logger"
)

var (
	reExpat = regexp.MustCompile(`\bExpat\b`)
)

// ZipMatches returns true if the zip file headers from in matches
// any of the path patterns. This is typically used in mime type
// detection of zipped files, such as Python wheels.
func ZipMatches(in []byte, patterns ...string) bool {
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

// GetLicense examines the given file to determine its license.
func GetLicense(filename string) (string, error) {
	var license string

	cmd := []string{"licensecheck", "--machine", "--shortname-scheme=spdx", filename}
	logger.Debugf("check license: %v", cmd)
	out, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		return license, fmt.Errorf("license check error: %s", err)
	}

	fields := strings.Split(strings.TrimSpace(string(out)), "\t")
	if len(fields) > 1 {
		license = fields[1]
		// Debian uses "Expat" for SPDX MIT license
		license = reExpat.ReplaceAllLiteralString(license, "MIT")
	}

	return license, nil
}
