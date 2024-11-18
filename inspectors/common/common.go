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
	ErrRejectedArtefact = errors.New("artefact rejected by inspectors")
)

// ArtefactReader represents the downloaded artefact file.
type ArtefactReader interface {
	io.ReadSeeker
	io.ReaderAt
	Len() int
}

// RequestArtefact is an interface with methods to be used on the
// artefact metadata during the request inspection.
type RequestArtefact interface {
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
	HTTPRequest() *http.Request

	// Save request for inspection
	SetRequestBody(io.ReadCloser)
}

// ResponseArtefact is an interface with methods to be used on the
// artefact metadata during the response inspection.
type ResponseArtefact interface {
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

	// Get downloaded artefact fields
	MimetypeIs(string) bool
	Size() int64
	Sha256() digests.Sha256Digest
	ContentType() string
	DownloadURL() string

	// Helpers to use during inspection
	CacheDir() string

	// Fill metadata fields
	SetArtefactMetadata(ArtefactMetadata)
}

// Inspector is the interface implemented by artefact metadata extractors.
type Inspector interface {
	ID() string

	InspectRequest(RequestArtefact) error

	// Inspect extracts metadata from the given artefact and
	// populates the metadata structure, returning whether
	// the artefact was identified and no further examination
	// by other inspectors is required.
	InspectArtefact(ArtefactReader, ResponseArtefact) error
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

// Inspection contains an opinion about the artefact set by an
// inspector. The inspector can also provide a free-form reason
// explaining its decision and annotations containing additional
// information.
type Inspection struct {
	Opinion     opinions.OpinionKind `json:"opinion"`
	Reason      string               `json:"reason"`
	Annotations Annotation           `json:"annotations,omitempty"`
}

// Annotate adds an annotation to the artefact's inspection.
func (in *Inspection) Annotate(a Annotation) {
	if in.Annotations == nil {
		in.Annotations = make(map[string]any, len(a))
	}
	for key := range a { // shallow copy the map
		in.Annotations[key] = a[key]
	}
}

// ArtefactMetadata contains essential metadata information
// about the artefact being inspected.
type ArtefactMetadata struct {
	Type         string // The type of the artefact file
	Name         string // The artefact designation, given by its author
	Version      string // The artefact version, as published by the upstream
	Vendor       string // The artefact vendor
	Description  string // A free-form description of the artefact
	Author       string // The artefact author name
	AuthorEmail  string // The artefact author email address
	Architecture string // The architecture, if the artefact contains binary code
	License      string // The license the artefact is published under
	Copyright    string // The copyright line, if available
}
