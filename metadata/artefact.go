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
	"fmt"
	"io"
	"net/http"

	"github.com/gabriel-vasile/mimetype"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
)

type InspectionMap map[string]*Inspection

// Artefact

const (
	MetadataVersionMajor = 0 // Updated when incompatible changes are made
	MetadataVersionMinor = 1 // Existing fields not changed, may contain additional fields
)

// Artefact holds information about each downloaded file during
// a build session.
type Artefact struct {
	MetadataVersion    string               `json:"artefact-metadata-version"` // Artefact metadata version in X.Y format
	RequestInspection  InspectionMap        `json:"request-inspection"`        // Opinions from request inspection
	ResponseInspection InspectionMap        `json:"response-inspection"`       // Opinions from result and artefact inspection
	Result             opinions.OpinionKind `json:"result"`                    // Inspection result
	Metadata           Metadata             `json:"metadata"`                  // Artefact metadata
	Downloads          []Download           `json:"downloads"`                 // Information about artefact downloads
	CurrentDownload    Download             `json:"-"`                         // Information about the current download
	AssetDir           string               `json:"-"`                         // Location to store files and metadata
	Tempfile           string               `json:"-"`                         // Path to temporary file containing downloaded data
	SessionId          string               `json:"-"`                         // The current session ID
	MimeType           *mimetype.MIME       `json:"-"`                         // The artefact MIME type
	Request            *http.Request        `json:"-"`                         // request handle for body content inspection

}

func NewArtefact() *Artefact {
	return &Artefact{
		MetadataVersion:    fmt.Sprintf("%d.%d", MetadataVersionMajor, MetadataVersionMinor),
		RequestInspection:  InspectionMap{},
		ResponseInspection: InspectionMap{},
		Metadata:           Metadata{},
		Downloads:          []Download{},
		CurrentDownload:    Download{},
		Request:            nil,
	}
}

// Implement RequestArtefact and ResponseArtefact

func (a *Artefact) RequestHeader(key string) ([]string, bool) {
	val, ok := a.CurrentDownload.RequestHeader[key]
	return val, ok
}

func (a *Artefact) ContentType() string {
	return a.CurrentDownload.ContentType
}

func (a *Artefact) DownloadURL() string {
	return a.CurrentDownload.URL
}

func (a *Artefact) HTTPRequest() *http.Request {
	return a.Request
}

func (a *Artefact) SetRequestBody(r io.ReadCloser) {
	a.Request.Body = r
}

func (a *Artefact) SetArtefactMetadata(m ArtefactMetadata) {
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
}

func (a *Artefact) MimetypeIs(t string) bool {
	if a.Metadata.Type == t {
		return true
	}
	return a.MimeType != nil && a.MimeType.Is(t)
}

func (a Artefact) Size() int64 {
	return a.Metadata.Size
}

func (a Artefact) Sha256() digests.Sha256Digest {
	return a.Metadata.Sha256
}

// addInspection adds the inspector's opinion to the artefact's
// inspection map.
func (a *Artefact) addInspection(insp InspectionMap, inspName, id string, op opinions.OpinionKind, reason string, args ...any) *Inspection {
	logger.Infof("%s: %s opinion set to %s (%s)", id, inspName, op.String(), reason)
	in := &Inspection{
		Opinion: op,
		Reason:  fmt.Sprintf(reason, args...),
	}
	insp[id] = in

	return in
}

// SetRequestPending adds a request inspection and sets the inspector
// ins opinion to Pending.
func (a *Artefact) SetRequestPending(ins Inspector, reason string, args ...any) *Inspection {
	return a.addInspection(a.RequestInspection, "request", ins.ID(), opinions.Pending, reason, args...)
}

// SetRequestRejected adds a request inspection and sets the inspector
// ins opinion to Rejected.
func (a *Artefact) SetRequestRejected(ins Inspector, reason string, args ...any) *Inspection {
	return a.addInspection(a.RequestInspection, "request", ins.ID(), opinions.Rejected, reason, args...)
}

// SetRequestUnknown adds a request inspection and sets the inspector
// ins opinion to Unknown.
func (a *Artefact) SetRequestUnknown(ins Inspector, reason string, args ...any) *Inspection {
	return a.addInspection(a.RequestInspection, "request", ins.ID(), opinions.Unknown, reason, args...)
}

// SetResponseApproved adds a response inspection and sets the inspector
// ins opinion to Approved.
func (a *Artefact) SetResponseApproved(ins Inspector, reason string, args ...any) *Inspection {
	return a.addInspection(a.ResponseInspection, "response", ins.ID(), opinions.Approved, reason, args...)
}

// SetResponseRejected adds a response inspection and sets the inspector
// ins opinion to Rejected.
func (a *Artefact) SetResponseRejected(ins Inspector, reason string, args ...any) *Inspection {
	return a.addInspection(a.ResponseInspection, "response", ins.ID(), opinions.Rejected, reason, args...)
}

// SetResponseUnknown adds a response inspection and sets the inspector
// ins opinion to Unknown.
func (a *Artefact) SetResponseUnknown(ins Inspector, reason string, args ...any) *Inspection {
	return a.addInspection(a.ResponseInspection, "response", ins.ID(), opinions.Unknown, reason, args...)
}

// RequestRejected returns true when the artefact was rejected
// during request inspection.
func (a *Artefact) RequestRejected() bool {
	for _, in := range a.RequestInspection {
		if in.Opinion == opinions.Rejected {
			return true
		}
	}
	return false
}

// RequestPending returns true when the artefact was not rejected
// during request inspection and there's at least one pending opinion.
func (a *Artefact) RequestPending() bool {
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

// ResponseRejected returns true when the artefact was rejected
// during artefact inspection.
func (a *Artefact) ResponseRejected() bool {
	for _, in := range a.ResponseInspection {
		if in.Opinion == opinions.Rejected {
			return true
		}
	}
	return false
}

// ResponseApproved returns true when the artefact was not rejected
// during response inspection and there's at least one approval.
func (a *Artefact) ResponseApproved() bool {
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
func (a *Artefact) RequestAnnotation(id, key string) (any, bool) {
	var def any = nil
	return inspectionAnnotation(a.RequestInspection, id, key, def)
}

// RequestStringAnnotation verifies whether the inspector has a request
// opinion and returns its annotation value if it's a string. If the
// inspector and annotation key are valid and the annotation type is
// correct ok returns true, otherwise it returns false.
func (a *Artefact) RequestStringAnnotation(id, key string) (string, bool) {
	var def string = ""
	return inspectionAnnotation(a.RequestInspection, id, key, def)
}

// RequestBoolAnnotation verifies whether the inspector has a request
// opinion and returns its annotation value if it's bool. If the inspector
// and annotation key are valid and the annotation type is correct ok
// returns true, otherwise it returns false.
func (a *Artefact) RequestBoolAnnotation(id, key string) (bool, bool) {
	var def bool = false
	return inspectionAnnotation(a.RequestInspection, id, key, def)
}

// ResponseAnnotation verifies whether the inspector has a response
// opinion and returns its annotation value. If the inspector and
// annotation key are valid ok returns true, otherwise it returns false.
func (a *Artefact) ResponseAnnotation(id, key string) (any, bool) {
	var def any = nil
	return inspectionAnnotation(a.ResponseInspection, id, key, def)
}

// ResponseStringAnnotation verifies whether the inspector has a response
// opinion and returns its annotation value if it's a string. If the inspector
// and annotation key are valid and the annotation type is correct ok returns
// true, otherwise it returns false.
func (a *Artefact) ResponseStringAnnotation(id, key string) (string, bool) {
	var def string = ""
	return inspectionAnnotation(a.ResponseInspection, id, key, def)
}

// ResponseBoolAnnotation verifies whether the inspector has a response
// opinion and returns its annotation value if it's bool. If the inspector
// and annotation key are valid and the annotation type is correct ok
// returns true, otherwise it returns false.
func (a *Artefact) ResponseBoolAnnotation(id, key string) (bool, bool) {
	var def bool = false
	return inspectionAnnotation(a.ResponseInspection, id, key, def)
}

// Approved returns true when the request was set to pending and there's at
// ueast one approval opinion and no rejections in the response inspection.
func (a *Artefact) Approved() bool {
	if !a.RequestPending() {
		return false
	}
	return a.ResponseApproved()
}

// Rejected returns the opposite of Approved.
func (a *Artefact) Rejected() bool {
	return !a.Approved()
}
