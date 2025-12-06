// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2024 Canonical Ltd.
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

package testutils

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

func CreateZip(output, input string) error {
	z, err := os.Create(output)
	if err != nil {
		return err
	}
	defer func() { _ = z.Close() }()

	zw := zip.NewWriter(z)
	defer func() { _ = zw.Close() }()

	return filepath.Walk(input, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// make path relative to archive root
		zpath, err := filepath.Rel(input, path)
		if err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()

		zf, err := zw.Create(zpath)
		if err != nil {
			return err
		}

		_, err = io.Copy(zf, f)

		return err
	})
}
