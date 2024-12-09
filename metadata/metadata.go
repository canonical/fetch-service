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

package metadata

import (
	"time"

	. "github.com/canonical/fetch-service/metadata/digests"
)

// Metadata holds information about each artifact.
type Metadata struct {
	Type         string       `json:"type"`                   // The mime-type of the artifact file
	Sha1         Sha1Digest   `json:"sha1"`                   // The SHA1 digest of the artifact file
	Sha256       Sha256Digest `json:"sha256"`                 // The SHA256 digest of the artifact file
	Size         int64        `json:"size"`                   // The size of the artifact file
	Name         string       `json:"name"`                   // The artifact designation, given by its author
	Version      string       `json:"version"`                // The artifact version, as published by the upstream
	Vendor       string       `json:"vendor"`                 // The artifact vendor
	Description  string       `json:"description"`            // A free-form description of the artifact
	Author       string       `json:"author"`                 // The artifact author name
	AuthorEmail  string       `json:"author-email,omitempty"` // The artifact author email address
	Architecture string       `json:"architecture,omitempty"` // The architecture, if the artifact contains binary code
	License      string       `json:"license"`                // The license the artifact is published under
	Copyright    string       `json:"copyright,omitempty"`    // The copyright line, if available
}

// Download holds information about each artifact download.
type Download struct {
	StartTime      time.Time           `json:"start-time"`      // When the downloaded started (UTC)
	EndTime        time.Time           `json:"end-time"`        // When the download finished (UTC)
	Method         string              `json:"method"`          // The HTTP request method
	URL            string              `json:"url"`             // The requested URL
	Address        string              `json:"address"`         // The HTTP client's IP address
	UserAgent      string              `json:"user-agent"`      // The HTTP client's user agent
	RequestHeader  map[string][]string `json:"request-header"`  // The HTTP request header
	StatusCode     int                 `json:"status-code"`     // The HTTP response status code
	Status         string              `json:"status"`          // The HTTP response status message
	ContentType    string              `json:"content-type"`    // The HTTP content type
	ResponseHeader map[string][]string `json:"response-header"` // The HTTP response header
	Sha256         Sha256Digest        `json:"-"`               // SHA256 digest of the downloaded data
}

// SessionMetadata holds information about each session.
type SessionMetadata struct {
	Generator  string    `json:"generator"`         // The name of the generator of this metadata
	Comment    string    `json:"comment,omitempty"` // Free-form comment text
	SessionId  string    `json:"session-id"`        // The unique session ID
	StartTime  time.Time `json:"start-time"`        // When the session started (UTC)
	EndTime    time.Time `json:"end-time"`          // When the session finished (UTC)
	Inspectors []string  `json:"inspectors"`        // A list of registered inspector IDs
	SpoolPath  string    `json:"spool-path"`        // The filesystem path to session artifacts
	Policy     string    `json:"policy"`            // Session policy (strict or permissive)
	Err        error     `json:"-"`
}

// SessionInfo contains brief information to be listed in service status.
type SessionInfo struct {
	SessionId string `json:"session-id"` // session ID
	StartTime string `json:"start-time"` // session start timestamp
	Policy    string `json:"policy"`     // session policy ("strict" or "permissive")
	Age       uint64 `json:"age"`        // session age in seconds
	Timeout   uint64 `json:"timeout"`    // session timeout in seconds
}
