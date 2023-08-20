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

type AnnotationValue map[string]interface{}

// Annotation contains a free-form text added by an artifact
// inspector.
type Annotation struct {
	Timestamp time.Time       `json:"time"`            // When the annotation was added
	Value     AnnotationValue `json:"value,omitempty"` // Optional annotation value
}

type AnnotationMap map[string]*Annotation

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

// Metadata holds information about each artifact.
type Metadata struct {
	MetadataVersion string         `json:"metadata-version"`       // Metadata version in X.Y format
	Type            string         `json:"type"`                   // The mime-type of the artifact file
	Sha1            Sha1Digest     `json:"sha1"`                   // The SHA1 digest of the artifact file
	Sha256          Sha256Digest   `json:"sha256"`                 // The SHA256 digest of the artifact file
	Size            int64          `json:"size"`                   // The size of the artifact file
	Name            string         `json:"name"`                   // The artifact designation, given by its author
	Version         string         `json:"version"`                // The artifact version, as published by the upstream
	Vendor          string         `json:"vendor"`                 // The artifact vendor
	Description     string         `json:"description"`            // A free-form description of the artifact
	Author          string         `json:"author"`                 // The artifact author name
	AuthorEmail     string         `json:"author-email,omitempty"` // The artifact author email address
	Architecture    string         `json:"architecture,omitempty"` // The architecture, if the artifact contains binary code
	License         string         `json:"license"`                // The license the artifact is published under
	Copyright       string         `json:"copyright,omitempty"`    // The copyright line, if available
	Annotations     AnnotationMap  `json:"annotations,omitempty"`  // Annotations added by artifact inspectors
	Downloads       []DownloadInfo `json:"downloads"`              // Information about artifact downloads
	Files           []MemberFile   `json:"files,omitempty"`        // Information about files contained in this artifact
	AssetDir        string         `json:"-"`                      // Location to store files and metadata
	Tempfile        string         `json:"-"`                      // Path to temporary file containing downloaded data
}

// Annotate adds a named annotation to the file metadata.
func (md *Metadata) Annotate(name string, value AnnotationValue) *Annotation {
	a := &Annotation{time.Now().UTC(), value}
	if md.Annotations == nil {
		md.Annotations = AnnotationMap{}
	}
	md.Annotations[name] = a

	return a
}

// MemberFile contains information about files contained in the artifact.
type MemberFile struct {
	Name   string       `json:"name"`   // The qualified file name
	Sha256 Sha256Digest `json:"sha256"` // The SHA256 digest of content
	Size   int64        `json:"size"`   // The file size
}

// DownloadInfo holds information about each artifact download.
type DownloadInfo struct {
	StartTime      time.Time           `json:"start-time"`      // When the downloaded started (UTC)
	EndTime        time.Time           `json:"end-time"`        // When the download finished (UTC)
	Method         string              `json:"method"`          // The HTTP request method
	URL            string              `json:"url"`             // The requested URL
	Address        string              `json:"address"`         // The HTTP client's IP address
	UserAgent      string              `json:"user-agent"`      // The HTTP client's user agent
	StatusCode     int                 `json:"status-code"`     // The HTTP response status code
	Status         string              `json:"status"`          // The HTTP response status message
	ContentType    string              `json:"content-type"`    // The HTTP content type
	ResponseHeader map[string][]string `json:"response-header"` // The HTTP response header
	Sha256         Sha256Digest        `json:"-"`               // SHA256 digest of the downloaded data
	SessionId      string              `json:"-"`               // The current session ID
}

// FileDownload has the metadata of a downloaded file and details
// about the download operation.
type FileDownload struct {
	Rch  chan error   // Handler response channel
	Md   Metadata     // Downloaded file metadata
	Info DownloadInfo // Download operation details
}

type DownloadAuthorizationRequest struct {
	Rch  chan error   // Handler response channel
	Info DownloadInfo // Download operation details
}

func NewFileDownload(md Metadata, info DownloadInfo) FileDownload {
	return FileDownload{
		Rch:  make(chan error, 1),
		Md:   md,
		Info: info,
	}
}
