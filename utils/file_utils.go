// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2025 Canonical Ltd.
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
	"fmt"
	"io"
	"os"
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
func ZipMatches(in io.ReaderAt, size int64, patterns []*regexp.Regexp) bool {
	z, err := zip.NewReader(in, size)
	if err != nil {
		return false
	}

	num := len(patterns)

	for _, f := range z.File {
		for _, p := range patterns {
			if matched := p.MatchString(f.Name); matched {
				num--
				if num == 0 {
					// all patterns found
					return true
				}
			}
		}
	}

	return false
}

// GetLicense examines the given file to determine its license.
func GetLicense(filename string, slog logger.Logger) (string, error) {
	var license string

	cmd := []string{"licensecheck", "--machine", "--shortname-scheme=spdx", filename}
	slog.Debugf("check license: %v", cmd)
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

// CheckLicenseFiles examines common files to determine the license.
func CheckLicenseFiles(files []string, slog logger.Logger) (string, error) {
	for _, f := range files {
		_, err := os.Stat(f)
		if err != nil {
			continue
		}

		license, err := GetLicense(f, slog)
		if err != nil {
			return "", err
		}

		return license, nil
	}

	return "", nil
}

var osRename = osRenameImpl

func osRenameImpl(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// MoveFile renames a file or recreates it at the destination if
// renaming is not possible.
func MoveFile(oldpath, newpath string) error {
	// Try renaming first
	if err := osRename(oldpath, newpath); err == nil {
		return nil
	}

	oldfile, err := os.Open(oldpath)
	if err != nil {
		return err
	}
	defer oldfile.Close()

	// Open old file
	fi, err := oldfile.Stat()
	if err != nil {
		return err
	}

	// Open new file
	perm := fi.Mode() & os.ModePerm
	newfile, err := os.OpenFile(newpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer newfile.Close()

	// Move file content
	if _, err := io.Copy(newfile, oldfile); err != nil {
		newfile.Close()
		os.Remove(newpath)
		return err
	}

	// Remove old file
	if err := os.Remove(oldpath); err != nil {
		return err
	}

	return nil
}
