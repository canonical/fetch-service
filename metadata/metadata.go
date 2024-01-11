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
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

const (
	MetadataVersionMajor = 0 // Updated when incompatible changes are made
	MetadataVersionMinor = 1 // Existing fields not changed, may contain additional fields
)

// Sha1Digest contains a 120-bit SHA1 digest.
type Sha1Digest [20]byte

func NewSha1Digest(digest string) (Sha1Digest, error) {
	h, err := hex.DecodeString(digest)
	if err != nil {
		return Sha1Digest{}, err
	}
	if len(h) != 20 { // SHA1 digest length is 160 bits
		return Sha1Digest{}, fmt.Errorf("SHA1 digest length (%d) is invalid", len(h))
	}
	return *(*Sha1Digest)(h), nil
}

// String obtains the SHA1 digest value as a hexadecimal string.
func (h Sha1Digest) String() string {
	return hex.EncodeToString(h[:])
}

// MarshalJSON marshals a SHA1 digest as a hexadecimal string.
func (h Sha1Digest) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(h.String())), nil
}

// UnmarshalJSON unmarshals a hexadecimal string representation of
// a SHA1 digest back to binary format.
func (h *Sha1Digest) UnmarshalJSON(data []byte) (err error) {
	d, err := strconv.Unquote(string(data))
	if err != nil {
		return
	}

	v, err := hex.DecodeString(d)
	if err != nil {
		return
	}

	copy((*h)[:], v)
	return
}

// Sha256Digest contains a 256-bit SHA1 digest
type Sha256Digest [32]byte

func NewSha256Digest(digest string) (Sha256Digest, error) {
	h, err := hex.DecodeString(digest)
	if err != nil {
		return Sha256Digest{}, err
	}
	if len(h) != 32 { // SHA256 digest length is 256 bits
		return Sha256Digest{}, fmt.Errorf("SHA256 digest length (%d) is invalid", len(h))
	}
	return *(*Sha256Digest)(h), nil
}

// String obtains the SHA256 digest value as a hexadecimal string.
func (h Sha256Digest) String() string {
	return hex.EncodeToString(h[:])
}

// MarshalJSON marshals a SHA256 digest as a hexadecimal string.
func (h Sha256Digest) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(h.String())), nil
}

// UnmarshalJSON unmarshals a hexadecimal string representation of
// a SHA256 digest back to binary format.
func (h *Sha256Digest) UnmarshalJSON(data []byte) (err error) {
	d, err := strconv.Unquote(string(data))
	if err != nil {
		return
	}

	v, err := hex.DecodeString(d)
	if err != nil {
		return
	}

	copy((*h)[:], v)
	return
}

// Metadata holds information about each artefact.
type Metadata struct {
	MetadataVersion string       `json:"metadata-version"`       // Metadata version in X.Y format
	Type            string       `json:"type"`                   // The mime-type of the artefact file
	Sha1            Sha1Digest   `json:"sha1"`                   // The SHA1 digest of the artefact file
	Sha256          Sha256Digest `json:"sha256"`                 // The SHA256 digest of the artefact file
	Size            int64        `json:"size"`                   // The size of the artefact file
	Name            string       `json:"name"`                   // The artefact designation, given by its author
	Version         string       `json:"version"`                // The artefact version, as published by the upstream
	Vendor          string       `json:"vendor"`                 // The artefact vendor
	Description     string       `json:"description"`            // A free-form description of the artefact
	Author          string       `json:"author"`                 // The artefact author name
	AuthorEmail     string       `json:"author-email,omitempty"` // The artefact author email address
	Architecture    string       `json:"architecture,omitempty"` // The architecture, if the artefact contains binary code
	License         string       `json:"license"`                // The license the artefact is published under
	Copyright       string       `json:"copyright,omitempty"`    // The copyright line, if available
}

// Download holds information about each artefact download.
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
	SessionId  string    `json:"session-id"` // The unique session ID
	StartTime  time.Time `json:"start-time"` // When the session started (UTC)
	EndTime    time.Time `json:"end-time"`   // When the session finished (UTC)
	Inspectors []string  `json:"inspectors"` // A list of registered inspector IDs
	SpoolPath  string    `json:"spool-path"` // The filesystem path to session artefacts
}
