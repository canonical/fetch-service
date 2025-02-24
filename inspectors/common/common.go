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

package common

import (
	"errors"
	"io"
	"net/http"

	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
)

var (
	ErrRejectedRequest  = errors.New("request rejected by inspectors")
	ErrRejectedArtifact = errors.New("artifact rejected by inspectors")
)

// ArtifactReader represents the downloaded artifact file.
type ArtifactReader interface {
	io.ReadSeeker
	io.ReaderAt
	Len() int
}

// RequestArtifact is an interface with methods to be used on the
// artifact metadata during the request inspection.
type RequestArtifact interface {
	// Inspector opinions
	SetRequestPending(Inspector, string, ...any) *Inspection
	SetRequestRejected(Inspector, string, ...any) *Inspection
	SetRequestUnknown(Inspector, string, ...any) *Inspection
	RequestPending() bool
	RequestRejected() bool

	// Get annotations
	RequestAnnotation(string, string) (any, bool)
	RequestStringAnnotation(string, string) (string, bool)
	RequestBoolAnnotation(string, string) (bool, bool)

	// Get request fields
	DownloadURL() string
	RequestHeader(string) ([]string, bool)
	RequestHeaderContains(string, string) bool
	HTTPRequest() *http.Request

	// Save request for inspection
	SetRequestBody(io.ReadCloser)
}

// ResponseArtifact is an interface with methods to be used on the
// artifact metadata during the response inspection.
type ResponseArtifact interface {
	// Inspector opinions
	SetResponseApproved(Inspector, string, ...any) *Inspection
	SetResponseRejected(Inspector, string, ...any) *Inspection
	SetResponseUnknown(Inspector, string, ...any) *Inspection
	ResponseApproved() bool
	ResponseRejected() bool

	// Get annotations
	RequestAnnotation(string, string) (any, bool)
	RequestStringAnnotation(string, string) (string, bool)
	RequestBoolAnnotation(string, string) (bool, bool)
	ResponseAnnotation(string, string) (any, bool)
	ResponseStringAnnotation(string, string) (string, bool)
	ResponseBoolAnnotation(string, string) (bool, bool)

	// Get downloaded artifact fields
	MimetypeIs(string) bool
	Size() int64
	Sha256() digests.Sha256Digest
	ContentType() string
	DownloadURL() string

	// Helpers to use during inspection
	CacheDir() string

	// Fill metadata fields
	SetArtifactMetadata(ArtifactMetadata)
}

// Inspector is the interface implemented by artifact metadata extractors.
type Inspector interface {
	ID() string

	InspectRequest(RequestArtifact) error

	// Inspect extracts metadata from the given artifact and
	// populates the metadata structure, returning whether
	// the artifact was identified and no further examination
	// by other inspectors is required.
	InspectArtifact(ArtifactReader, ResponseArtifact) error
}

// Annotation is a registry of free-form entries defined by the
// inspector.
type Annotation map[string]any

// Add adds a free-form value val to the specified Annotation key.
func (ann Annotation) Add(key string, val any) {
	ann[key] = val
}

// Append merges two Annotation maps.
func (ann Annotation) Append(more Annotation) {
	for key, val := range more {
		ann[key] = val
	}
}

// Inspection contains an opinion about the artifact set by an
// inspector. The inspector can also provide a free-form reason
// explaining its decision and annotations containing additional
// information.
type Inspection struct {
	Opinion     opinions.OpinionKind `json:"opinion"`
	Reason      string               `json:"reason"`
	Annotations Annotation           `json:"annotations,omitempty"`
}

// Annotate adds an annotation to the artifact's inspection.
func (in *Inspection) Annotate(a Annotation) {
	if in.Annotations == nil {
		in.Annotations = make(map[string]any, len(a))
	}
	for key := range a { // shallow copy the map
		in.Annotations[key] = a[key]
	}
}

// ArtifactMetadata contains essential metadata information
// about the artifact being inspected.
type ArtifactMetadata struct {
	Type          string // The type of the artifact file
	Name          string // The artifact designation, given by its author
	Version       string // The artifact version, as published by the upstream
	Vendor        string // The artifact vendor
	Description   string // A free-form description of the artifact
	Author        string // The artifact author name
	AuthorEmail   string // The artifact author email address
	Architecture  string // The architecture, if the artifact contains binary code
	License       string // The license the artifact is published under
	Copyright     string // The copyright line, if available
	SourcePackage string // The name of the source package that generated this artifact, if available.
	StoreRevision string // The revision of the artifact assigned by the store, if any.
}
