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

type OpinionKind int

const (
	Unknown OpinionKind = iota
	Rejected
	Approved
)

type Opinion struct {
	InspectorID string
	Opinion     OpinionKind
	Reason      string
}

type Artefact struct {
	opinions []Opinion
	metadata Metadata
	request  *DownloadInfo
}

func NewArtefact(di *DownloadInfo) *Artefact {
	return &Artefact{
		opinions: []Opinion{},
		metadata: Metadata{},
		request:  di,
	}
}

func (a *Artefact) Reject(id string, reason string) {
	o := Opinion{
		InspectorID: id,
		Opinion:     Rejected,
		Reason:      reason,
	}
	a.opinions = append(a.opinions, o)
}

func (a *Artefact) Approve(id string, reason string) {
	o := Opinion{
		InspectorID: id,
		Opinion:     Approved,
		Reason:      reason,
	}
	a.opinions = append(a.opinions, o)
}

func (a *Artefact) Approved() bool {
	if len(a.opinions) == 0 {
		return false
	}
	for _, o := range a.opinions {
		if o.Opinion != Approved {
			return false
		}
	}
	return true
}

func (a *Artefact) Rejected() bool {
	for _, o := range a.opinions {
		if o.Opinion == Rejected {
			return true
		}
	}
	return false
}

func (a *Artefact) Unknown() bool {
	for _, o := range a.opinions {
		if o.Opinion != Unknown {
			return false
		}
	}
	return true
}

func (a *Artefact) Opinions() []Opinion {
	return a.opinions
}

func (a *Artefact) ArtefactMetadata() *Metadata {
	return &a.metadata
}

func (a *Artefact) RequestMetadata() *DownloadInfo {
	return a.request
}
