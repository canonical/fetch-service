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
	"os"
)

var (
	GetGitProtocol    = getGitProtocol
	DecodeGitProtocol = decodeGitProtocol
)

func MockOsStat(mock func(string) (os.FileInfo, error)) (restorer func()) {
	old := osStat
	osStat = mock
	return func() {
		osStat = old
	}
}
