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
	"fmt"
	"net/http"

	"github.com/gabriel-vasile/mimetype"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata/opinions"
)

// Inspection

type InspectionState string

const (
	InitialState  = "Waiting for inspection"
	RequestState  = "Inspecting request"
	ResponseState = "Inspecting Response"
	ApprovedState = "Approved"
	RejectedState = "Rejected"
)

type Annotation map[string]any

func (ann Annotation) Add(key string, val any) {
	ann[key] = val
}

func (ann Annotation) Append(more Annotation) {
	for key, val := range more {
		ann[key] = val
	}
}

type Inspection struct {
	Opinion     opinions.OpinionKind `json:"opinion"`
	Reason      string               `json:"reason"`
	Annotations Annotation           `json:"annotations,omitempty"`
}

func (in *Inspection) Annotate(a Annotation) {
	if in.Annotations == nil {
		in.Annotations = make(map[string]any, len(a))
	}
	for key := range a { // shallow copy the map
		in.Annotations[key] = a[key]
	}
}

type InspectionMap map[string]*Inspection

// Artefact

const (
	MetadataVersionMajor = 0 // Updated when incompatible changes are made
	MetadataVersionMinor = 1 // Existing fields not changed, may contain additional fields
)

type Artefact struct {
	MetadataVersion    string               `json:"metadata-version"`    // Metadata version in X.Y format
	RequestInspection  InspectionMap        `json:"request-inspection"`  // Opinions from request inspection
	ResponseInspection InspectionMap        `json:"response-inspection"` // Opinions from result and artefact inspection
	Result             opinions.OpinionKind `json:"result"`              // Inspection result
	Metadata           Metadata             `json:"metadata"`            // Artefact metadata
	Downloads          []Download           `json:"downloads"`           // Information about artefact downloads
	CurrentDownload    Download             `json:"-"`                   // Information about the current download
	AssetDir           string               `json:"-"`                   // Location to store files and metadata
	Tempfile           string               `json:"-"`                   // Path to temporary file containing downloaded data
	SessionId          string               `json:"-"`                   // The current session ID
	MimeType           *mimetype.MIME       `json:"-"`                   // The artefact MIME type
	Request            *http.Request        `json:"-"`                   // request handle for body content inspection
}

func NewArtefact() *Artefact {
	return &Artefact{
		MetadataVersion:    fmt.Sprintf("%d.%d", MetadataVersionMajor, MetadataVersionMinor),
		RequestInspection:  InspectionMap{},
		ResponseInspection: InspectionMap{},
		Metadata:           Metadata{},
		Downloads:          []Download{},
		CurrentDownload:    Download{},
	}
}

// SetResponseOpinion adds the inspector's opinion to the artefact's
// request inspection map.
func (a *Artefact) SetRequestOpinion(id string, op opinions.OpinionKind, reason string, args ...any) *Inspection {
	// Valid request opinions are Unknown, Rejected and Pending
	if op != opinions.Unknown && op != opinions.Rejected && op != opinions.Pending {
		logger.Fatalf("%s: cannot set request opinion to %v, rejecting", id, op)
		op = opinions.Rejected
	}

	logger.Infof("%s: request opinion set to %v", id, op)
	in := &Inspection{
		Opinion: op,
		Reason:  fmt.Sprintf(reason, args...),
	}
	a.RequestInspection[id] = in

	return in
}

// SetResponseOpinion adds the inspector's opinion to the artefact's
// response inspection map.
func (a *Artefact) SetResponseOpinion(id string, op opinions.OpinionKind, reason string, args ...any) *Inspection {
	// Valid response opinions are Unknown, Rejected and Approved
	if op != opinions.Unknown && op != opinions.Rejected && op != opinions.Approved {
		logger.Errorf("%s: cannot set response opinion to %v, rejecting", id, op)
		op = opinions.Rejected
	}

	logger.Infof("%s: response opinion set to %v", id, op)
	in := &Inspection{
		Opinion: op,
		Reason:  fmt.Sprintf(reason, args...),
	}
	a.ResponseInspection[id] = in

	return in

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
// during request inspection and there's at least one opinion pending.
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

// Approved returns true when there's at least one approval opinion
// and no rejections in the response inspection.
func (a *Artefact) Approved() bool {
	if a.RequestRejected() {
		return false
	}
	res := false
	for _, in := range a.ResponseInspection {
		if in.Opinion == opinions.Rejected {
			return false
		} else if in.Opinion == opinions.Approved {
			res = true
		}
	}
	return res
}

// Rejected returns the opposite of Approved.
func (a *Artefact) Rejected() bool {
	return !a.Approved()
}
