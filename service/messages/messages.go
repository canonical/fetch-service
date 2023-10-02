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

package messages

import (
	"github.com/canonical/fetch-service/metadata"
)

// ArtefactDownloadMessage has the metadata of a downloaded file and details
// about the download operation.
type ArtefactDownload struct {
	Rch chan error         // Handler response channel
	A   *metadata.Artefact // Artefact and download metadata
}

func NewArtefactDownload(a *metadata.Artefact) ArtefactDownload {
	return ArtefactDownload{
		Rch: make(chan error, 1),
		A:   a,
	}
}

type RequestAuthorization struct {
	Rch chan error         // Handler response channel
	A   *metadata.Artefact // Artefact and download metadata
}

func NewRequestAuthorization(a *metadata.Artefact) RequestAuthorization {
	return RequestAuthorization{
		Rch: make(chan error, 1),
		A:   a,
	}
}

// Session creation

type SessionCredentials struct {
	Id     string `json:"id"`
	Pw     string `json:"pw"`
	Policy string `json:"policy"`
}

type CreateSession struct {
	Rch chan SessionCredentials // Handler response channel
}

func NewCreateSession() CreateSession {
	return CreateSession{
		Rch: make(chan SessionCredentials, 1),
	}
}

// Session end

type SessionResult struct {
	Err error
	// TODO: add session stats
	Artefacts []*metadata.Artefact `json:"artefacts"`
}

type EndSession struct {
	Rch chan SessionResult // Handler response channel
	Id  string
}

func NewEndSession(sessionId string) EndSession {
	return EndSession{
		Rch: make(chan SessionResult, 1),
		Id:  sessionId,
	}
}
