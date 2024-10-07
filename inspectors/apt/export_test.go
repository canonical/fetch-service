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
	"io"

	. "github.com/canonical/fetch-service/inspectors/common"
)

type AptPackagesEntry = aptPackagesEntry
type ReleaseFile = releaseFile
type ReleaseEntry = releaseEntry

var (
	DecodePublicKey = decodePublicKey
	ParsePackages   = parsePackages
)

func MockCheckSignature(mock func(io.ReadSeeker, Annotation, string) (io.ReadSeeker, error)) (restorer func()) {
	old := checkSignature
	checkSignature = mock
	return func() {
		checkSignature = old
	}
}

func (ins *AptReleaseInspector) Release() map[string]ReleaseFile {
	return ins.release
}

func (ins *AptReleaseInspector) SetRelease(release map[string]ReleaseFile) {
	ins.release = release
}
