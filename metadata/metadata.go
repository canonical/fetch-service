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
	"time"
)

// FileInfo holds information about each artifact.
type FileInfo struct {
	Type   string `json:"type"`
	Sha1   string `json:"sha1"`
	Sha256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// DownloadInfo holds information about each artifact download.
type DownloadInfo struct {
	StartTime      time.Time           `json:"start-time"`
	EndTime        time.Time           `json:"end-time"`
	Method         string              `json:"method"`
	URL            string              `json:"url"`
	UserAgent      string              `json:"user-agent"`
	StatusCode     int                 `json:"status-code"`
	Status         string              `json:"status"`
	ContentType    string              `json:"content-type"`
	ResponseHeader map[string][]string `json:"response-header"`
	Size           int64               `json:"size"`
	Digest         string              `json:"digest"`
	SessionId      string              `json:"session-id"`
}
