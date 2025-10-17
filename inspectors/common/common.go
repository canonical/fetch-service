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

// Package common provides the core interfaces and types for the artifact inspection system.
// Inspectors analyze download requests and downloaded artifacts to extract metadata,
// validate content, and make security decisions about whether to approve or reject artifacts.
package common

import (
	"errors"
	"io"
	"net/http"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
)

var (
	// ErrRejectedRequest indicates that one or more inspectors rejected the download request
	// before the artifact was downloaded.
	ErrRejectedRequest = errors.New("request rejected by inspectors")

	// ErrRejectedArtifact indicates that one or more inspectors rejected the downloaded
	// artifact after inspection.
	ErrRejectedArtifact = errors.New("artifact rejected by inspectors")
)

// ArtifactReader provides read access to a downloaded artifact file.
// It combines standard Go interfaces for flexible file access patterns
// needed by different types of inspectors.
type ArtifactReader interface {
	io.ReadSeeker // Sequential and seekable reading
	io.ReaderAt   // Random access reading at specific offsets
	Len() int     // Total size of the artifact in bytes
}

// RequestArtifact provides methods for inspectors to analyze and make decisions
// about download requests before the artifact is actually downloaded.
// Inspectors can examine the URL, headers, and other request metadata to decide
// whether the download should proceed.
type RequestArtifact interface {
	// Inspector opinion methods - used to record decisions about the request
	SetRequestPending(Inspector, string, ...any) *Inspection  // Mark request as requiring further inspection
	SetRequestRejected(Inspector, string, ...any) *Inspection // Reject and prevent the download
	SetRequestUnknown(Inspector, string, ...any) *Inspection  // Mark as unrecognized/neutral
	RequestPending() bool                                     // Check if any inspector marked request as pending
	RequestRejected() bool                                    // Check if any inspector rejected the request

	// Annotation retrieval methods - access data stored by inspectors
	RequestAnnotation(string, string) (any, bool)          // Get annotation value by inspector ID and key
	RequestStringAnnotation(string, string) (string, bool) // Get string annotation value
	RequestBoolAnnotation(string, string) (bool, bool)     // Get boolean annotation value

	// Request metadata access
	DownloadURL() string                       // The URL being downloaded
	RequestHeader(string) ([]string, bool)     // Get HTTP header values
	RequestHeaderContains(string, string) bool // Check if header contains specific value
	HTTPRequest() *http.Request                // Access to the full HTTP request

	// Request body handling
	SetRequestBody(io.ReadCloser) // Store request body for inspection

	// Logging interface
	Logger() logger.Logger // Get logger instance for this request
}

// ResponseArtifact provides methods for inspectors to analyze downloaded artifacts
// and make final approval/rejection decisions. Inspectors can examine the actual
// file content, verify checksums, extract metadata, and decide whether the
// artifact should be accepted or rejected.
type ResponseArtifact interface {
	// Inspector opinion methods - used to record final decisions about the artifact
	SetResponseApproved(Inspector, string, ...any) *Inspection // Approve the artifact for use
	SetResponseRejected(Inspector, string, ...any) *Inspection // Reject the artifact
	SetResponseUnknown(Inspector, string, ...any) *Inspection  // Mark as unrecognized/neutral
	ResponseApproved() bool                                    // Check if any inspector approved the artifact
	ResponseRejected() bool                                    // Check if any inspector rejected the artifact

	// Annotation retrieval methods - access data from both request and response phases
	RequestAnnotation(string, string) (any, bool)           // Get request-phase annotation
	RequestStringAnnotation(string, string) (string, bool)  // Get request-phase string annotation
	RequestBoolAnnotation(string, string) (bool, bool)      // Get request-phase boolean annotation
	ResponseAnnotation(string, string) (any, bool)          // Get response-phase annotation
	ResponseStringAnnotation(string, string) (string, bool) // Get response-phase string annotation
	ResponseBoolAnnotation(string, string) (bool, bool)     // Get response-phase boolean annotation

	// Downloaded artifact properties
	MimetypeIs(string) bool       // Check if artifact matches specific MIME type
	Size() int64                  // Size of downloaded artifact in bytes
	Sha256() digests.Sha256Digest // SHA256 hash of the artifact content
	ContentType() string          // HTTP Content-Type header value
	DownloadURL() string          // Original download URL

	// Inspector utilities
	CacheDir() string // Directory for temporary files during inspection

	// Metadata population
	SetArtifactMetadata(ArtifactMetadata) // Set extracted metadata for the artifact

	// Logging interface
	Logger() logger.Logger // Get logger instance for this artifact
}

// Inspector defines the interface that all artifact inspectors must implement.
// Inspectors are responsible for analyzing download requests and downloaded artifacts
// to extract metadata, validate content, and make security decisions.
//
// The inspection process happens in two phases:
// 1. Request inspection (before download) - analyze URL, headers, etc.
// 2. Artifact inspection (after download) - analyze the actual file content
type Inspector interface {
	// ID returns a unique identifier for this inspector.
	// The ID is used to namespace annotations and track which inspector
	// made specific decisions about an artifact.
	ID() string

	// InspectRequest analyzes a download request before the artifact is downloaded.
	// Inspectors can examine the URL, HTTP headers, and other request metadata
	// to decide whether the download should proceed. The inspector should call
	// SetRequestPending, SetRequestRejected, or SetRequestUnknown on the artifact
	// to record its decision.
	InspectRequest(RequestArtifact) error

	// InspectArtifact analyzes a downloaded artifact to extract metadata and
	// make final approval/rejection decisions. The inspector has access to the
	// actual file content through ArtifactReader and can examine file structure,
	// verify checksums, extract metadata, etc. The inspector should call
	// SetResponseApproved, SetResponseRejected, or SetResponseUnknown to record
	// its final decision about the artifact.
	InspectArtifact(ArtifactReader, ResponseArtifact) error
}

// Annotation is a map of key-value pairs that inspectors use to store
// additional information about their analysis. Annotations allow inspectors
// to provide structured data beyond their basic approve/reject decision,
// such as extracted metadata, validation details, or security findings.
type Annotation map[string]any

// Add stores a key-value pair in the annotation map.
// If the key already exists, its value will be overwritten.
func (ann Annotation) Add(key string, val any) {
	ann[key] = val
}

// Append merges another Annotation map into this one.
// If keys overlap, values from the 'more' map will overwrite existing values.
func (ann Annotation) Append(more Annotation) {
	for key, val := range more {
		ann[key] = val
	}
}

// Inspection represents the result of an inspector's analysis of an artifact.
// It contains the inspector's opinion (approve, reject, unknown, or pending),
// a human-readable reason for the decision, and optional structured annotations
// with additional details about the analysis.
type Inspection struct {
	Opinion     opinions.OpinionKind `json:"opinion"`               // The inspector's decision
	Reason      string               `json:"reason"`                // Human-readable explanation
	Annotations Annotation           `json:"annotations,omitempty"` // Additional structured data
}

// Annotate adds annotations to this inspection result.
// If annotations don't exist yet, a new map is created.
// If the inspection already has annotations, the new ones are merged in,
// with new values overwriting existing keys.
func (in *Inspection) Annotate(a Annotation) {
	if in.Annotations == nil {
		in.Annotations = make(map[string]any, len(a))
	}
	for key := range a { // shallow copy the map
		in.Annotations[key] = a[key]
	}
}

// ArtifactMetadata contains standardized metadata fields that inspectors
// can extract from artifacts. This provides a consistent structure for
// common artifact properties across different file types and sources.
// Not all fields are applicable to every artifact type.
type ArtifactMetadata struct {
	Type          string // The type/format of the artifact file (e.g., "snap", "deb", "container-image")
	Name          string // The artifact name as designated by its author
	Version       string // The artifact version as published by the upstream source
	Vendor        string // The organization or individual that published the artifact
	Description   string // A free-form description of the artifact's purpose
	Author        string // The name of the artifact's author/creator
	AuthorEmail   string // Email address of the artifact's author
	Architecture  string // Target architecture for binary artifacts (e.g., "amd64", "arm64")
	License       string // The license under which the artifact is published
	Copyright     string // Copyright notice, if available
	SourcePackage string // Name of the source package that generated this artifact
	StoreRevision string // Revision number assigned by a store or registry
}
