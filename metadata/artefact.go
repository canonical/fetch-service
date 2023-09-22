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

type Identifiable interface {
	ID() string
}

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
	Opinions        []Opinion      `json:"opinions"`
	Metadata        Metadata       `json:"metadata"`
	Downloads       []DownloadInfo `json:"downloads"` // Information about artifact downloads
	CurrentDownload DownloadInfo   `json:"-"`         // Information about the current download
	AssetDir        string         `json:"-"`         // Location to store files and metadata
	Tempfile        string         `json:"-"`         // Path to temporary file containing downloaded data
	Sha256          Sha256Digest   `json:"-"`         // SHA256 digest of the downloaded data
	SessionId       string         `json:"-"`         // The current session ID
}

func NewArtefact() *Artefact {
	return &Artefact{
		Opinions:        []Opinion{},
		Metadata:        Metadata{},
		Downloads:       []DownloadInfo{},
		CurrentDownload: DownloadInfo{},
	}
}

func (a *Artefact) Reject(id Identifiable, reason string) {
	o := Opinion{
		InspectorID: id.ID(),
		Opinion:     Rejected,
		Reason:      reason,
	}
	a.Opinions = append(a.Opinions, o)
}

func (a *Artefact) Approve(id Identifiable, reason string) {
	o := Opinion{
		InspectorID: id.ID(),
		Opinion:     Approved,
		Reason:      reason,
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

/*
func (a *Artefact) Opinions() []Opinion {
	return a.opinions
}

func (a *Artefact) ArtefactMetadata() *Metadata {
	return &a.metadata
}

func (a *Artefact) RequestMetadata() *DownloadInfo {
	return a.request
}
*/
