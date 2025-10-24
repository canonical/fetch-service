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

package messages

import (
	"errors"
	"time"

	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/secrets"
	"github.com/canonical/fetch-service/service/config"
)

// ProxyAuth contains credentials for basic authentication.
type ProxyAuth struct {
	Rch chan bool // return channel
	Id  string    // user (session id)
	Pw  string    // password
}

// Service status

type ServiceStatus struct {
	Uptime         uint64                 `json:"uptime"`          // time since session start in seconds
	StartTime      time.Time              `json:"start-time"`      // service creation time
	SessionCount   uint64                 `json:"session-count"`   // number of created sessions
	ActiveSessions []metadata.SessionInfo `json:"active-sessions"` // list of active sessions
}

// Errors
var (
	ErrSessionNotFound     = errors.New("session not found")
	ErrSessionActive       = errors.New("session token is active")
	ErrSessionNotFinished  = errors.New("session not finished")
	ErrInvalidSessionToken = errors.New("invalid session token")
)

func NewGetServiceStatus() GetServiceStatus {
	return GetServiceStatus{
		Rch: make(chan ServiceStatus, 1),
	}
}

type GetServiceStatus struct {
	Rch chan ServiceStatus
}

// Inspection

type RequestInspection struct {
	Rch chan error         // Handler response channel
	A   *metadata.Artifact // Artifact and download metadata
}

func NewRequestInspection(a *metadata.Artifact) RequestInspection {
	return RequestInspection{
		Rch: make(chan error, 1),
		A:   a,
	}
}

type ResponseInspection struct {
	Rch chan error         // Handler response channel
	A   *metadata.Artifact // Artifact and download metadata
}

func NewResponseInspection(a *metadata.Artifact) ResponseInspection {
	return ResponseInspection{
		Rch: make(chan error, 1),
		A:   a,
	}
}

type CompleteInspection struct {
	Rch chan error         // Handler response channel
	A   *metadata.Artifact // Artifact and download metadata
}

func NewCompleteInspection(a *metadata.Artifact) CompleteInspection {
	return CompleteInspection{
		Rch: make(chan error, 1),
		A:   a,
	}
}

// Session creation

type SessionCredentials struct {
	Id    string `json:"id"`
	Token string `json:"token"`
	Err   error  `json:"-"`
}

type CreateSession struct {
	Rch              chan SessionCredentials        // Handler response channel
	Timeout          uint64                         // Session timeout in seconds
	Policy           string                         // Session policy (strict or permissive)
	Secrets          []secrets.Secret               // Secrets for the session
	InspectorsConfig config.SessionInspectorsConfig // Session inspectors configuration
}

func NewCreateSession(policy string, timeout uint64, secrets []secrets.Secret, inspectorsConfig config.SessionInspectorsConfig) CreateSession {
	return CreateSession{
		Rch:              make(chan SessionCredentials, 1),
		Policy:           policy,
		Timeout:          timeout,
		Secrets:          secrets,
		InspectorsConfig: inspectorsConfig,
	}
}

// Revoke session token

type RevokeToken struct {
	Rch   chan RevokeTokenResult // Handler response channel
	Id    string                 // The session ID
	Token string                 // The session token to revoke
}

func NewRevokeToken(sessionId, token string) RevokeToken {
	return RevokeToken{
		Rch:   make(chan RevokeTokenResult, 1),
		Id:    sessionId,
		Token: token,
	}
}

type RevokeTokenResult struct {
	SessionId string `json:"session-id"`
	StartTime string `json:"start-time"`
	EndTime   string `json:"end-time"`
	SpoolPath string `json:"spool-path"`
	Err       error  `json:"-"`
}

// Session report

type SessionReport struct {
	Rch chan SessionReportResult // Handler response channel
	Id  string
}

func NewSessionReport(sessionId string) SessionReport {
	return SessionReport{
		Rch: make(chan SessionReportResult, 1),
		Id:  sessionId,
	}
}

type SessionReportResult struct {
	*metadata.SessionMetadata
	Artifacts []*metadata.Artifact `json:"artifacts"`
	Err       error                `json:"-"`
}

// Session end

type EndSession struct {
	Rch chan error // Handler response channel
	Id  string
}

func NewEndSession(sessionId string) EndSession {
	return EndSession{
		Rch: make(chan error, 1),
		Id:  sessionId,
	}
}

// Delete resources

type DeleteResources struct {
	Rch chan error // Handler response channel
	Id  string
}

func NewDeleteResources(sessionId string) DeleteResources {
	return DeleteResources{
		Rch: make(chan error, 1),
		Id:  sessionId,
	}
}

// Fetchctl messages

type FetchCtl struct {
	Rch          chan FetchCtlResult
	Operation    string
	Type         string
	ValidateOnly bool
	Payload      []byte
}

func NewFetchCtl(operation, optype string, validateOnly bool, payload []byte) FetchCtl {
	return FetchCtl{
		Operation:    operation,
		Type:         optype,
		ValidateOnly: validateOnly,
		Payload:      payload,
		Rch:          make(chan FetchCtlResult, 1),
	}
}

type FetchCtlResult struct {
	Status  string
	Message string
}

type CreateSessionPayload struct {
	SessionId        string `json:"session-id"`
	Token            string `json:"token"`
	Timeout          int    `json:"timeout"`
	Mode             string `json:"mode"`
	InspectorsConfig []byte `json:"inspectors-configuration"`
}
