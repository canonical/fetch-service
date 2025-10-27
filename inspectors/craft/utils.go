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

package craft

import (
	"os"

	. "github.com/canonical/fetch-service/inspectors/common"
)

const (
	GitUploadPackID = "git.upload-pack"
)

var osStat = os.Stat
var osOpen = os.Open

func checkGitRequestHeaders(a RequestArtifact) bool {
	return a.RequestHeaderContains("Content-Type", "application/x-git-upload-pack-request") &&
		a.RequestHeaderContains("Accept", "application/x-git-upload-pack-result")
}

func getSingleFetchedRef(a ResponseArtifact) string {
	wants, ok := a.RequestAnnotation("git.upload-pack", "wants")
	if !ok {
		return ""
	}

	list, ok := wants.([]string)
	if !ok || len(list) != 1 {
		return ""
	}

	return list[0]
}
