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
	"errors"
	"fmt"

	"github.com/gabriel-vasile/mimetype"

	"github.com/canonical/fetch-service/logger"
)

type OpinionKind int

const (
	Unknown OpinionKind = iota
	Rejected
	Approved
	Pending
)

func (t OpinionKind) MarshalJSON() ([]byte, error) {
	switch t {
	case Unknown:
		return []byte(`"Unknown"`), nil
	case Rejected:
		return []byte(`"Rejected"`), nil
	case Approved:
		return []byte(`"Approved"`), nil
	case Pending:
		return []byte(`"Pending"`), nil
	default:
		return nil, errors.New("invalid opinion kind")
	}
}

func (t *OpinionKind) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"Unknown"`:
		*t = Unknown
		return nil
	case `"Rejected"`:
		*t = Rejected
		return nil
	case `"Approved"`:
		*t = Approved
		return nil
	default:
		return errors.New("invalid opinion kind")
	}
}

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
	Opinion     OpinionKind `json:"opinion"`
	Reason      string      `json:"reason"`
	Annotations Annotation  `json:"annotations,omitempty"`
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

type Artefact struct {
	RequestInspection  InspectionMap   `json:"request-inspection"`  // Opinions from request inspection
	ResponseInspection InspectionMap   `json:"response-inspection"` // Opinions from result and artefact inspection
	State              InspectionState `json:"result"`              // Final inspection result
	Metadata           Metadata        `json:"metadata"`            // Artefact metadata
	Downloads          []Download      `json:"downloads"`           // Information about artefact downloads
	CurrentDownload    Download        `json:"-"`                   // Information about the current download
	AssetDir           string          `json:"-"`                   // Location to store files and metadata
	Tempfile           string          `json:"-"`                   // Path to temporary file containing downloaded data
	SessionId          string          `json:"-"`                   // The current session ID
	MimeType           *mimetype.MIME  `json:"-"`                   // The artefact MIME type
}

func NewArtefact() *Artefact {
	return &Artefact{
		RequestInspection:  InspectionMap{},
		ResponseInspection: InspectionMap{},
		State:              InitialState,
		Metadata:           Metadata{},
		Downloads:          []Download{},
		CurrentDownload:    Download{},
	}
}

type Identifiable interface {
	ID() string
}

func (a *Artefact) ConsideredBy(name string) bool {
	in, ok := a.RequestInspection[name]
	return ok && in.Opinion == Pending
}

func (a *Artefact) Consider(id Identifiable, reason string, args ...any) *Inspection {
	logger.Infof("request authorized by inspector %q", id.ID())
	in := &Inspection{
		Opinion: Pending,
		Reason:  fmt.Sprintf(reason, args...),
	}

	a.RequestInspection[id.ID()] = in

	return in
}

func (a *Artefact) Reject(id Identifiable, reason string, args ...any) *Inspection {
	in := &Inspection{
		Opinion: Rejected,
		Reason:  fmt.Sprintf(reason, args...),
	}

	if a.State == ResponseState {
		a.ResponseInspection[id.ID()] = in
	} else {
		a.RequestInspection[id.ID()] = in
	}

	return in

}

func (a *Artefact) Approve(id Identifiable, reason string, args ...any) *Inspection {
	in := &Inspection{
		Opinion: Approved,
		Reason:  fmt.Sprintf(reason, args...),
	}

	a.ResponseInspection[id.ID()] = in

	return in
}

func (a *Artefact) Pending() bool {
	if len(a.RequestInspection) == 0 {
		return false
	}
	for _, in := range a.RequestInspection {
		if in.Opinion == Pending {
			return true
		}
	}
	return false
}

func (a *Artefact) Approved() bool {
	for _, in := range a.RequestInspection {
		if in.Opinion == Rejected {
			return false
		}
	}

	if len(a.ResponseInspection) == 0 {
		return false
	}
	for _, in := range a.ResponseInspection {
		if in.Opinion != Approved {
			return false
		}
	}
	return true
}

func (a *Artefact) Rejected() bool {
	for _, in := range a.RequestInspection {
		if in.Opinion == Rejected {
			return true
		}
	}
	for _, in := range a.ResponseInspection {
		if in.Opinion == Rejected {
			return true
		}
	}
	return false
}

func (a *Artefact) Unknown() bool {
	for _, in := range a.ResponseInspection {
		if in.Opinion != Unknown {
			return false
		}
	}
	return true
}
