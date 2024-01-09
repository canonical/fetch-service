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

	"github.com/canonical/fetch-service/logger"
)

type OpinionKind int

const (
	Unknown OpinionKind = iota
	Rejected
	Approved
)

func (t OpinionKind) MarshalJSON() ([]byte, error) {
	switch t {
	case Unknown:
		return []byte(`"Unknown"`), nil
	case Rejected:
		return []byte(`"Rejected"`), nil
	case Approved:
		return []byte(`"Approved"`), nil
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

type Opinion struct {
	InspectorID string      `json:"inspector-id"`
	Opinion     OpinionKind `json:"opinion"`
	Reason      string      `json:"reason"`
}

type Artefact struct {
	Opinions        []Opinion           `json:"opinions"`
	Metadata        Metadata            `json:"metadata"`
	Downloads       []Download          `json:"downloads"` // Information about artefact downloads
	CurrentDownload Download            `json:"-"`         // Information about the current download
	AssetDir        string              `json:"-"`         // Location to store files and metadata
	Tempfile        string              `json:"-"`         // Path to temporary file containing downloaded data
	SessionId       string              `json:"-"`         // The current session ID
	ApprovedReqs    map[string]struct{} `json:"-"`         // Inspector IDs with approved requests
}

func NewArtefact() *Artefact {
	return &Artefact{
		Opinions:        []Opinion{},
		Metadata:        Metadata{},
		Downloads:       []Download{},
		CurrentDownload: Download{},
		ApprovedReqs:    map[string]struct{}{},
	}
}

type Identifiable interface {
	ID() string
}

func (a *Artefact) ApproveRequest(id Identifiable) {
	logger.Infof("request approved by inspector %q", id.ID())
	a.ApprovedReqs[id.ID()] = struct{}{}
}

func (a *Artefact) Reject(id Identifiable, reason string, args ...interface{}) {
	o := Opinion{
		InspectorID: id.ID(),
		Opinion:     Rejected,
		Reason:      fmt.Sprintf(reason, args...),
	}
	a.Opinions = append(a.Opinions, o)
}

func (a *Artefact) Approve(id Identifiable, reason string, args ...interface{}) {
	o := Opinion{
		InspectorID: id.ID(),
		Opinion:     Approved,
		Reason:      fmt.Sprintf(reason, args...),
	}
	a.Opinions = append(a.Opinions, o)
}

func (a *Artefact) Approved() bool {
	if len(a.Opinions) == 0 {
		return false
	}
	for _, o := range a.Opinions {
		if o.Opinion != Approved {
			return false
		}
	}
	return true
}

func (a *Artefact) Rejected() bool {
	for _, o := range a.Opinions {
		if o.Opinion == Rejected {
			return true
		}
	}
	return false
}

func (a *Artefact) Unknown() bool {
	for _, o := range a.Opinions {
		if o.Opinion != Unknown {
			return false
		}
	}
	return true
}
