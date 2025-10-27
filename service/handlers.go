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

package service

import (
	"fmt"
	"os"
	"time"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/canonical/fetch-service/secrets"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
)

func handleMessages(svc *Service, msg interface{}) {
	switch v := msg.(type) {
	case messages.GetServiceStatus:
		v.Rch <- messages.ServiceStatus{
			Uptime:         uint64(time.Since(svc.start).Seconds()),
			StartTime:      svc.start,
			SessionCount:   svc.totalSessions,
			ActiveSessions: session.SessionInfos(),
		}

	case messages.RequestInspection:
		handleRequestInspection(v)

	case messages.ResponseInspection:
		handleResponseInspection(v, svc.ch)

	case messages.CompleteInspection:
		handleCompleteInspection(v)

	case messages.CreateSession:
		handleCreateSession(v, svc.opt.Spool, svc.opt.PermissiveMode, svc)

	case messages.RevokeToken:
		handleRevokeToken(v, svc.opt.Spool)

	case messages.SessionReport:
		handleSessionReport(v)

	case messages.EndSession:
		handleEndSession(v)

	case messages.DeleteResources:
		handleDeleteResources(v, svc.opt.Spool)

	case messages.ProxyAuth:
		v.Rch <- session.CheckAuth(v.ID, v.Pw)

	case messages.FetchCtl:
		logger.Infof("service: fetchctl operation: %s", v.Operation)
		reply := handleFetchCtl(v, svc)
		v.Rch <- reply

	default:
		logger.Warningf("service: unknown message type %T", v)
	}
}

func handleRequestInspection(v messages.RequestInspection) {
	sessionID := v.A.SessionID

	s := session.GetSession(sessionID)
	if s == nil {
		v.Rch <- fmt.Errorf("cannot inspect request: session %s is not active", sessionID)
		return
	}

	// Run request inspectors
	go func(s *session.Session, a *metadata.Artifact) {
		v.Rch <- runRequestInspection(s, a)
	}(s, v.A)
}

func handleResponseInspection(v messages.ResponseInspection, ch chan interface{}) {
	sessionID := v.A.SessionID
	digest := v.A.Metadata.Sha256
	slog := v.A.Logger()

	s := session.GetSession(sessionID)
	if s == nil {
		v.Rch <- fmt.Errorf("cannot inspect response: session %s is not active", sessionID)
		slog.Debugf("remove stale temporary file: %s", v.A.Tempfile)
		os.Remove(v.A.Tempfile)
		return
	}

	v.A.CurrentDownload.EndTime = time.Now().UTC()

	// Add download info to artifact metadata
	dl := v.A.CurrentDownload
	slog.Infof("%s %s: %s (%s)", dl.Method, dl.URL, dl.Status, dl.ContentType)

	// Reuse the result of a previous inspection if the artifact is already added
	// to the current session (meaning that there is a complete inspection of
	// the same artifact). Otherwise save the temporary file to the file spool
	// for inspection. Files are added to the spool only if they're not already
	// there.
	if s.HasArtifact(digest) {
		slog.Infof("artifact %s already downloaded", digest)
		s.AddDownload(v.A.CurrentDownload)
		os.Remove(v.A.Tempfile)
		if !s.Permissive && s.ArtifactResult(digest) == opinions.Rejected {
			v.Rch <- ErrRejectedArtifact
		} else {
			v.Rch <- nil
		}
		return
	} else {
		if err := s.SaveData(v.A); err != nil {
			v.Rch <- err
			return
		}
	}

	// No previous inspection of this artifact is complete, so run the response
	// inspectors on the artifact file saved to the spool. Inspectors run
	// asynchronously and will add the artifact to the session on completion. If
	// the artifact has already been added to the session, add a new download
	// entry to the existing artifact.
	go func(s *session.Session, a *metadata.Artifact, ch chan interface{}) {
		err := runResponseInspection(s, a)

		// Add artifact to session after inspection
		cinsp := messages.NewCompleteInspection(v.A)
		ch <- cinsp
		errCompletion := <-cinsp.Rch
		if errCompletion != nil {
			v.Rch <- errCompletion
			return
		}

		v.Rch <- err

	}(s, v.A, ch)
}

func handleCompleteInspection(v messages.CompleteInspection) {
	digest := v.A.Metadata.Sha256
	s := session.GetSession(v.A.SessionID)
	if s == nil {
		v.Rch <- fmt.Errorf("cannot complete inspection: session %s is not active", v.A.SessionId)
		return
	}
	if !s.HasArtifact(digest) {
		s.AddArtifact(v.A)
	}
	s.AddDownload(v.A.CurrentDownload)

	slog := v.A.Logger()
	slog.Infof("artifact %s inspection complete", digest)
	v.Rch <- nil
}

func handleCreateSession(v messages.CreateSession, spoolDir string, permissiveMode bool, svc *Service) {
	permissive := false
	if v.Policy == "permissive" {
		if permissiveMode {
			permissive = true
		} else {
			v.Rch <- messages.SessionCredentials{
				Err: session.ErrInvalidSessionPolicy,
			}
			return
		}
	}

	timeout := time.Duration(v.Timeout * uint64(time.Second))
	sec := v.Secrets
	if err := secrets.ValidateSecrets(sec); err != nil {
		v.Rch <- messages.SessionCredentials{
			Err: err,
		}
		return
	}
	s := session.New(spoolDir, timeout, permissive, sec, v.InspectorsConfig)
	svc.totalSessions++
	v.Rch <- messages.SessionCredentials{ID: s.ID, Token: s.Token}
}

func handleRevokeToken(v messages.RevokeToken, spoolDir string) {
	sessionID := v.ID
	s := session.GetSession(sessionID)
	if s == nil {
		v.Rch <- messages.RevokeTokenResult{
			Err: messages.ErrSessionNotFound,
		}
		return
	}

	if !s.Revoke(v.Token) {
		v.Rch <- messages.RevokeTokenResult{
			Err: messages.ErrInvalidSessionToken,
		}
		return
	}

	v.Rch <- messages.RevokeTokenResult{
		SessionID: s.ID,
		StartTime: s.Start.String(),
		EndTime:   s.End.String(),
		SpoolPath: spoolDir,
	}
}

func handleSessionReport(v messages.SessionReport) {
	sessionID := v.ID
	s := session.GetSession(sessionID)
	if s == nil {
		v.Rch <- messages.SessionReportResult{
			SessionMetadata: &metadata.SessionMetadata{Err: messages.ErrSessionNotFound},
			Artifacts:       []*metadata.Artifact{},
			Err:             messages.ErrSessionNotFound,
		}
		return
	}

	if !s.IsRevoked() {
		err := fmt.Errorf("cannot get session report: session %s token was not revoked", sessionID)
		v.Rch <- messages.SessionReportResult{
			SessionMetadata: &metadata.SessionMetadata{Err: err},
			Artifacts:       []*metadata.Artifact{},
			Err:             messages.ErrSessionActive,
		}
		return
	}

	v.Rch <- messages.SessionReportResult{
		SessionMetadata: s.Metadata(),
		Artifacts:       s.Artifacts(),
	}
}

func handleEndSession(v messages.EndSession) {
	sessionID := v.ID
	s := session.GetSession(sessionID)
	if s == nil {
		v.Rch <- messages.ErrSessionNotFound
		return
	}

	v.Rch <- s.Finish()
}

func handleDeleteResources(v messages.DeleteResources, spoolDir string) {
	sessionID := v.ID
	s := session.GetSession(sessionID)
	if s != nil {
		v.Rch <- messages.ErrSessionNotFinished
		return
	}

	// Delete session resources
	go func(spoolDir, sessionID string) {
		v.Rch <- session.RemoveResources(spoolDir, sessionID)
	}(spoolDir, sessionID)
}

func runRequestInspection(s *session.Session, a *metadata.Artifact) error {
	// Check request
	if err := s.Insps.RunRequestInspectors(a); err != nil {
		a.Logger().Error(err.Error())
		return err
	}

	return evaluateRequestInspection(s, a)
}

func evaluateRequestInspection(s *session.Session, a *metadata.Artifact) error {
	dl := a.CurrentDownload
	slog := a.Logger()

	if !a.RequestPending() {
		if s.Permissive {
			slog.Infof("request would be rejected: %s %s", dl.Method, dl.URL)
		} else {
			slog.Infof("request rejected: %s %s", dl.Method, dl.URL)
			return ErrRejectedRequest
		}
	} else {
		slog.Infof("request approved: %s %s", dl.Method, dl.URL)
	}

	return nil
}

func runResponseInspection(s *session.Session, a *metadata.Artifact) error {
	// Extract metadata from file
	if err := s.Insps.RunArtifactInspectors(a.AssetDir, a); err != nil {
		a.Logger().Errorf("%s", err)
		a.Result = opinions.Rejected
		return err
	}

	return evaluateResponseInspection(s, a)
}

func evaluateResponseInspection(s *session.Session, a *metadata.Artifact) error {
	digest := a.Metadata.Sha256
	slog := a.Logger()

	if a.Rejected() {
		a.Result = opinions.Rejected
		if s.Permissive {
			slog.Infof("artifact %s %d (%s) would be rejected (permissive)", digest, a.Metadata.Size, a.Metadata.Type)
		} else {
			slog.Infof("artifact rejected: %s %d (%s)", digest, a.Metadata.Size, a.Metadata.Type)
			return ErrRejectedArtifact
		}
	} else {
		a.Result = opinions.Approved
		slog.Infof("artifact approved: %s %d (%s)", digest, a.Metadata.Size, a.Metadata.Type)
	}

	return nil
}
