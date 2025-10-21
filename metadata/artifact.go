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

package metadata

import (
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/gabriel-vasile/mimetype"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type InspectionMap map[string]*Inspection

// Artifact

const (
	MetadataVersionMajor = 0 // Updated when incompatible changes are made
	MetadataVersionMinor = 2 // Existing fields not changed, may contain additional fields
)

// Artifact holds information about each downloaded file during
// a build session.
type Artifact struct {
	MetadataVersion    string               `json:"artifact-metadata-version"` // Artifact metadata version in X.Y format
	RequestInspection  InspectionMap        `json:"request-inspection"`        // Opinions from request inspection
	ResponseInspection InspectionMap        `json:"response-inspection"`       // Opinions from result and artifact inspection
	Result             opinions.OpinionKind `json:"result"`                    // Inspection result
	Metadata           Metadata             `json:"metadata"`                  // Artifact metadata
	Downloads          []Download           `json:"downloads"`                 // Information about artifact downloads
	CurrentDownload    Download             `json:"-"`                         // Information about the current download
	AssetDir           string               `json:"-"`                         // Location to store files and metadata
	Tempfile           string               `json:"-"`                         // Path to temporary file containing downloaded data
	SessionId          string               `json:"-"`                         // The current session ID
	SessionCacheDir    string               `json:"-"`                         // Location to store files and metadata
	MimeType           *mimetype.MIME       `json:"-"`                         // The artifact MIME type
	Request            *http.Request        `json:"-"`                         // request handle for body content inspection

	logger logger.Logger `json:"-"` // Session-aware log helper
}

func NewArtifact() *Artifact {
	return &Artifact{
		MetadataVersion:    fmt.Sprintf("%d.%d", MetadataVersionMajor, MetadataVersionMinor),
		RequestInspection:  InspectionMap{},
		ResponseInspection: InspectionMap{},
		Metadata:           Metadata{},
		Downloads:          []Download{},
		CurrentDownload:    Download{},
		Request:            nil,
		logger:             logger.NewSessionLogger("no-session"),
	}
}

// Implement RequestArtifact and ResponseArtifact

func (a *Artifact) RequestHeader(key string) ([]string, bool) {
	val, ok := a.CurrentDownload.RequestHeader[key]
	return val, ok
}

// RequestHeaderContains returns true if the request header h contains
// string s.
func (a *Artifact) RequestHeaderContains(h, s string) bool {
	value, ok := a.RequestHeader(h)
	return ok && slices.Contains(value, s)
}

func (a *Artifact) ContentType() string {
	return a.CurrentDownload.ContentType
}

func (a *Artifact) DownloadURL() string {
	return a.CurrentDownload.URL
}

func (a *Artifact) HTTPRequest() *http.Request {
	return a.Request
}

func (a *Artifact) SetRequestBody(r io.ReadCloser) {
	a.Request.Body = r
}

func (a *Artifact) SetArtifactMetadata(m ArtifactMetadata) {
	if m.Type != "" {
		a.Metadata.Type = m.Type
	}
	a.Metadata.Name = m.Name
	a.Metadata.Version = m.Version
	a.Metadata.Vendor = m.Vendor
	a.Metadata.Description = m.Description
	a.Metadata.Author = m.Author
	a.Metadata.AuthorEmail = m.AuthorEmail
	a.Metadata.Architecture = m.Architecture
	a.Metadata.License = m.License
	a.Metadata.Copyright = m.Copyright
	a.Metadata.SourcePackage = m.SourcePackage
	a.Metadata.StoreRevision = m.StoreRevision
}

func (a *Artifact) MimetypeIs(t string) bool {
	if a.Metadata.Type == t {
		return true
	}
	return a.MimeType != nil && a.MimeType.Is(t)
}

func (a Artifact) Size() int64 {
	return a.Metadata.Size
}

func (a Artifact) Sha256() digests.Sha256Digest {
	return a.Metadata.Sha256
}

// addInspection adds the inspector's opinion to the artifact's
// inspection map.
func (a *Artifact) addInspection(insp InspectionMap, inspName, id string, op opinions.OpinionKind, reason string, args ...any) *Inspection {
	a.logger.Infof("%s: %s opinion set to %s (%s)", id, inspName, op.String(), reason)
	in := &Inspection{
		Opinion: op,
		Reason:  fmt.Sprintf(reason, args...),
	}
	insp[id] = in

	return in
}

// SetRequestPending adds a request inspection and sets the inspector
// ins opinion to Pending.
func (a *Artifact) SetRequestPending(ins Inspector, reason string, args ...any) *Inspection {
	return a.addInspection(a.RequestInspection, "request", ins.ID(), opinions.Pending, reason, args...)
}

// SetRequestRejected adds a request inspection and sets the inspector
// ins opinion to Rejected.
func (a *Artifact) SetRequestRejected(ins Inspector, reason string, args ...any) *Inspection {
	return a.addInspection(a.RequestInspection, "request", ins.ID(), opinions.Rejected, reason, args...)
}

// SetRequestUnknown adds a request inspection and sets the inspector
// ins opinion to Unknown.
func (a *Artifact) SetRequestUnknown(ins Inspector, reason string, args ...any) *Inspection {
	return a.addInspection(a.RequestInspection, "request", ins.ID(), opinions.Unknown, reason, args...)
}

// SetResponseApproved adds a response inspection and sets the inspector
// ins opinion to Approved.
func (a *Artifact) SetResponseApproved(ins Inspector, reason string, args ...any) *Inspection {
	return a.addInspection(a.ResponseInspection, "response", ins.ID(), opinions.Approved, reason, args...)
}

// SetResponseRejected adds a response inspection and sets the inspector
// ins opinion to Rejected.
func (a *Artifact) SetResponseRejected(ins Inspector, reason string, args ...any) *Inspection {
	return a.addInspection(a.ResponseInspection, "response", ins.ID(), opinions.Rejected, reason, args...)
}

// SetResponseUnknown adds a response inspection and sets the inspector
// ins opinion to Unknown.
func (a *Artifact) SetResponseUnknown(ins Inspector, reason string, args ...any) *Inspection {
	return a.addInspection(a.ResponseInspection, "response", ins.ID(), opinions.Unknown, reason, args...)
}

// RequestRejected returns true when the artifact was rejected
// during request inspection.
func (a *Artifact) RequestRejected() bool {
	for _, in := range a.RequestInspection {
		if in.Opinion == opinions.Rejected {
			return true
		}
	}
	return false
}

// RequestPending returns true when the artifact was not rejected
// during request inspection and there's at least one pending opinion.
func (a *Artifact) RequestPending() bool {
	res := false
	for _, in := range a.RequestInspection {
		if in.Opinion == opinions.Rejected {
			return false
		}
		if in.Opinion == opinions.Pending {
			res = true
		}
	}
	return res
}

// ResponseRejected returns true when the artifact was rejected
// during artifact inspection.
func (a *Artifact) ResponseRejected() bool {
	for _, in := range a.ResponseInspection {
		if in.Opinion == opinions.Rejected {
			return true
		}
	}
	return false
}

// ResponseApproved returns true when the artifact was not rejected
// during response inspection and there's at least one approval.
func (a *Artifact) ResponseApproved() bool {
	res := false
	for _, in := range a.ResponseInspection {
		if in.Opinion == opinions.Rejected {
			return false
		}
		if in.Opinion == opinions.Approved {
			res = true
		}
	}
	return res
}

// RequestOpinion returns the request opinion set by the inspector
// with the given ID.
func (a *Artifact) InspectorRequestOpinion(ins Inspector) opinions.OpinionKind {
	in, ok := a.RequestInspection[ins.ID()]
	if !ok {
		return opinions.Unknown
	}
	return in.Opinion
}

// inspectAnnotation verifies whether the inspector has an inspection
// opinion and returns its annotation or a default value.
func inspectionAnnotation[T any](insp InspectionMap, id, key string, def T) (T, bool) {
	inspection, ok := insp[id]
	if !ok {
		return def, false
	}
	ann, ok := inspection.Annotations[key]
	if !ok {
		return def, false
	}
	val, ok := ann.(T)
	if !ok {
		return def, false
	}
	return val, true
}

// RequestAnnotation verifies whether the inspector has a request
// opinion and returns its annotation value. If the inspector and
// annotation key are valid ok returns true, otherwise it returns false.
func (a *Artifact) RequestAnnotation(id, key string) (any, bool) {
	var def any = nil
	return inspectionAnnotation(a.RequestInspection, id, key, def)
}

// RequestStringAnnotation verifies whether the inspector has a request
// opinion and returns its annotation value if it's a string. If the
// inspector and annotation key are valid and the annotation type is
// correct ok returns true, otherwise it returns false.
func (a *Artifact) RequestStringAnnotation(id, key string) (string, bool) {
	var def string = ""
	return inspectionAnnotation(a.RequestInspection, id, key, def)
}

// RequestBoolAnnotation verifies whether the inspector has a request
// opinion and returns its annotation value if it's bool. If the inspector
// and annotation key are valid and the annotation type is correct ok
// returns true, otherwise it returns false.
func (a *Artifact) RequestBoolAnnotation(id, key string) (bool, bool) {
	var def bool = false
	return inspectionAnnotation(a.RequestInspection, id, key, def)
}

// ResponseAnnotation verifies whether the inspector has a response
// opinion and returns its annotation value. If the inspector and
// annotation key are valid ok returns true, otherwise it returns false.
func (a *Artifact) ResponseAnnotation(id, key string) (any, bool) {
	var def any = nil
	return inspectionAnnotation(a.ResponseInspection, id, key, def)
}

// ResponseStringAnnotation verifies whether the inspector has a response
// opinion and returns its annotation value if it's a string. If the inspector
// and annotation key are valid and the annotation type is correct ok returns
// true, otherwise it returns false.
func (a *Artifact) ResponseStringAnnotation(id, key string) (string, bool) {
	var def string = ""
	return inspectionAnnotation(a.ResponseInspection, id, key, def)
}

// ResponseBoolAnnotation verifies whether the inspector has a response
// opinion and returns its annotation value if it's bool. If the inspector
// and annotation key are valid and the annotation type is correct ok
// returns true, otherwise it returns false.
func (a *Artifact) ResponseBoolAnnotation(id, key string) (bool, bool) {
	var def bool = false
	return inspectionAnnotation(a.ResponseInspection, id, key, def)
}

// Approved returns true when the request was set to pending and there's at
// ueast one approval opinion and no rejections in the response inspection.
func (a *Artifact) Approved() bool {
	if !a.RequestPending() {
		return false
	}
	return a.ResponseApproved()
}

// Rejected returns the opposite of Approved.
func (a *Artifact) Rejected() bool {
	return !a.Approved()
}

func (a *Artifact) CacheDir() string {
	return a.SessionCacheDir
}

func (a *Artifact) Logger() logger.Logger {
	return a.logger
}

func (a *Artifact) SetLogger(l logger.Logger) {
	a.logger = l
}
