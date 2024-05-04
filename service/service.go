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

package service

import (
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"gopkg.in/tomb.v2"

	"github.com/canonical/fetch-service/control"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
)

// Service implements the fetch service main loop.
type Service struct {
	p     *proxy.HttpProxy // proxy instance
	ctl   *control.Server  // control server
	ch    chan interface{} // channel to get feedback from handlers
	start time.Time        // service start time (UTC)
	opt   *Options         // configuration options
	tomb  tomb.Tomb        // service dispacher loop reaper

	// statistics
	sCount            atomic.Uint64 // total number of sessions
	sErrors           atomic.Uint64 // total number of session errors
	sTime             time.Duration // cumulative session time
	sRequests         atomic.Uint64 // processed requests
	sApprovedRequests atomic.Uint64 // approved requests
	sRejectedRequests atomic.Uint64 // rejected requests
	sArtefacts        atomic.Uint64 // cumulative number of artefacts processed
	sApproved         atomic.Uint64 // approved artefacts
	sRejected         atomic.Uint64 // rejected artefacts
	sMaxTime          time.Duration // maximum session duration

}

var (
	proxyNewHttpProxy = proxy.NewHttpProxy
	controlNewServer  = control.NewServer
)

func New(opt *Options) *Service {
	// obtain authentication credentials from the environment
	creds := os.Getenv("FETCH_SERVICE_AUTH")

	ch := make(chan interface{})
	p := proxyNewHttpProxy(opt.ProxyPort, opt.Spool, ch)
	ctl := controlNewServer(opt.ControlPort, ch, creds)
	start := time.Now()

	return &Service{p: p, ctl: ctl, opt: opt, ch: ch, start: start}
}

// Start runs the fetch service dispatcher.
func (svc *Service) Start() error {
	logger.Info("Starting service...")
	if err := svc.p.Start(); err != nil {
		return err
	}

	svc.ctl.Start()

	svc.tomb.Go(func() error {
		for {
			select {
			case msg := <-svc.ch:
				switch v := msg.(type) {
				case messages.GetServiceStatus:
					var (
						avgTime time.Duration
						avgArts float32
						avgReqs float32
					)
					count := svc.sCount.Load()
					if count > 0 {
						avgTime = time.Duration(uint64(svc.sTime) / count)
						avgArts = float32(svc.sArtefacts.Load()) / float32(count)
						avgReqs = float32(svc.sRequests.Load()) / float32(count)
					}

					var mem runtime.MemStats
					runtime.ReadMemStats(&mem)

					v.Rch <- messages.ServiceStatus{
						Uptime:                     uint64(time.Since(svc.start).Seconds()),
						StartTime:                  svc.start,
						ActiveSessions:             session.ListAll(),
						SessionCount:               svc.sCount.Load(),
						SessionErrors:              svc.sErrors.Load(),
						TotalSessionTime:           uint64(svc.sTime.Seconds()),
						ProcessedRequests:          svc.sRequests.Load(),
						ApprovedRequests:           svc.sApprovedRequests.Load(),
						RejectedRequests:           svc.sRejectedRequests.Load(),
						ProcessedArtefacts:         svc.sArtefacts.Load(),
						ApprovedArtefacts:          svc.sApproved.Load(),
						RejectedArtefacts:          svc.sRejected.Load(),
						AverageArtefactsPerSession: avgArts,
						AverageRequestsPerSession:  avgReqs,
						AverageSessionTime:         float32(avgTime.Milliseconds()) / 1000,
						LongestSessionTime:         uint64(svc.sMaxTime.Seconds()),
						NumCPU:                     runtime.NumCPU(),
						NumRoutines:                runtime.NumGoroutine(),
						TotalMem:                   mem.Sys,
						Alloc:                      mem.Alloc,
					}

				case messages.RequestInspection:
					sessionId := v.A.SessionId

					s := session.GetSession(sessionId)
					if s == nil {
						svc.sErrors.Add(1)
						v.Rch <- fmt.Errorf("cannot inspect request: session %s is not active", sessionId)
						break
					}

					svc.sRequests.Add(1)
					s.NumRequests.Add(1)

					// Run request inspectors
					go func(svc *Service, s *session.Session, a *metadata.Artefact) {
						err := runRequestInspection(s, a)
						if a.Rejected() {
							svc.sRejectedRequests.Add(1)
							s.RejectedRequests.Add(1)
						} else {
							svc.sApprovedRequests.Add(1)
						}
						v.Rch <- err
					}(svc, s, v.A)

				case messages.ResponseInspection:
					sessionId := v.A.SessionId
					digest := v.A.Metadata.Sha256

					s := session.GetSession(sessionId)
					if s == nil {
						svc.sErrors.Add(1)
						v.Rch <- fmt.Errorf("cannot inspect response: session %s is not active", sessionId)
						break
					}

					svc.sArtefacts.Add(1)
					s.NumArtefacts.Add(1)
					v.A.CurrentDownload.EndTime = time.Now().UTC()

					// Add download info to artefact metadata
					dl := v.A.CurrentDownload
					logger.Infof("[%s] %s %s: %s (%s)", sessionId, dl.Method, dl.URL, dl.Status, dl.ContentType)

					if s.HasArtefact(digest) {
						logger.Infof("artefact %s already downloaded", digest)
						s.AddDownload(v.A.CurrentDownload)
						os.Remove(v.A.Tempfile)
						v.Rch <- nil
						break
					}

					// Add metadata to session
					s.AddArtefact(v.A)
					if err := s.SaveData(digest); err != nil {
						v.Rch <- err
						break
					}

					s.AddDownload(v.A.CurrentDownload)

					// Run response inspectors
					go func(svc *Service, s *session.Session, a *metadata.Artefact) {
						err := runResponseInspection(s, a)
						if a.Rejected() {
							svc.sRejected.Add(1)
							s.RejectedArtefacts.Add(1)
						} else {
							svc.sApproved.Add(1)
						}
						v.Rch <- err
					}(svc, s, v.A)

				case messages.CreateSession:
					permissive := false
					if v.Policy == "permissive" {
						if svc.opt.PermissiveMode {
							permissive = true
						} else {
							svc.sErrors.Add(1)
							v.Rch <- messages.SessionCredentials{
								Err: session.ErrInvalidSessionPolicy,
							}
							break
						}
					}

					s := session.New(svc.opt.Spool, permissive)
					if v.Timeout > 0 {
						s.Timeout = time.Duration(v.Timeout * uint64(time.Second))
					}
					v.Rch <- messages.SessionCredentials{Id: s.Id, Token: s.Token}
					svc.sCount.Add(1)

				case messages.RevokeToken:
					sessionId := v.Id
					s := session.GetSession(sessionId)
					if s == nil {
						svc.sErrors.Add(1)
						v.Rch <- messages.RevokeTokenResult{
							Err: messages.ErrSessionNotFound,
						}
						break
					}

					if !s.Revoke(v.Token) {
						v.Rch <- messages.RevokeTokenResult{
							Err: messages.ErrInvalidSessionToken,
						}
						break
					}

					// update stats
					sm := s.Metadata()
					duration := sm.EndTime.Sub(sm.StartTime)
					svc.sTime += duration
					if duration > svc.sMaxTime {
						svc.sMaxTime = duration
					}
					if sm.Err != nil {
						svc.sErrors.Add(1)
					}

					v.Rch <- messages.RevokeTokenResult{
						SessionId: s.Id,
						StartTime: s.Start.String(),
						SpoolPath: svc.opt.Spool,
					}

				case messages.SessionReport:
					sessionId := v.Id
					s := session.GetSession(sessionId)
					if s == nil {
						svc.sErrors.Add(1)
						v.Rch <- messages.SessionReportResult{
							SessionMetadata: &metadata.SessionMetadata{Err: messages.ErrSessionNotFound},
							Artefacts:       []*metadata.Artefact{},
							Err:             messages.ErrSessionNotFound,
						}
						break
					}

					if !s.IsRevoked() {
						svc.sErrors.Add(1)
						err := fmt.Errorf("cannot get session report: session %s token was not revoked", sessionId)
						v.Rch <- messages.SessionReportResult{
							SessionMetadata: &metadata.SessionMetadata{Err: err},
							Artefacts:       []*metadata.Artefact{},
							Err:             messages.ErrSessionActive,
						}
						break
					}

					v.Rch <- messages.SessionReportResult{
						SessionMetadata: s.Metadata(),
						Artefacts:       s.Artefacts(),
					}

				case messages.EndSession:
					sessionId := v.Id
					s := session.GetSession(sessionId)
					if s == nil {
						svc.sErrors.Add(1)
						v.Rch <- messages.ErrSessionNotFound
						break
					}

					v.Rch <- s.Finish()

				case messages.DeleteResources:
					sessionId := v.Id
					s := session.GetSession(sessionId)
					if s != nil {
						v.Rch <- messages.ErrSessionNotFinished
						break
					}

					// Delete session resources
					go func(spoolDir, sessionId string) {
						v.Rch <- session.RemoveResources(spoolDir, sessionId)
					}(svc.opt.Spool, sessionId)

				case messages.ProxyAuth:
					v.Rch <- session.CheckAuth(v.Id, v.Pw)

				default:
					logger.Warningf("Unknown message type %T", v)
				}

			case <-svc.tomb.Dying():
				return nil
			}

		}
	})

	return nil
}

func (svc *Service) Stop() error {
	logger.Info("Stopping service...")
	session.FinishAll()

	if err := svc.p.Stop(); err != nil {
		logger.Warningf("Cannot shut down the HTTP server: %s", err)
	}

	svc.tomb.Kill(nil)
	if err := svc.tomb.Wait(); err != nil {
		return err
	}

	return nil
}

func (svc *Service) Dying() <-chan struct{} {
	return svc.tomb.Dying()
}

func runRequestInspection(s *session.Session, a *metadata.Artefact) error {
	// Check request
	if err := s.Insps.RunRequestInspectors(a); err != nil {
		logger.Errorf("[%s] %s", s.Id, err)
		return err
	}

	dl := a.CurrentDownload
	sessionId := s.Id

	if a.Rejected() {
		if s.Permissive {
			logger.Infof("[%s] request would be rejected: %s %s", sessionId, dl.Method, dl.URL)
		} else {
			logger.Infof("[%s] request rejected: %s %s", sessionId, dl.Method, dl.URL)
			return ErrRejectedRequest
		}
	} else {
		logger.Infof("[%s] request approved: %s %s", sessionId, dl.Method, dl.URL)
	}

	return nil
}

func runResponseInspection(s *session.Session, a *metadata.Artefact) error {
	// Extract metadata from file
	if err := s.Insps.RunArtefactInspectors(a.AssetDir, a); err != nil {
		logger.Errorf("%s", err)
		return err
	}

	sessionId := s.Id
	digest := a.Metadata.Sha256

	if a.Rejected() {
		if s.Permissive {
			logger.Infof("[%s] artefact %s %d (%s) would be rejected (permissive)",
				sessionId, digest, a.Metadata.Size, a.Metadata.Type)
		} else {
			logger.Infof("[%s] artefact rejected: %s %d (%s)",
				sessionId, digest, a.Metadata.Size, a.Metadata.Type)
			return ErrRejectedArtefact
		}
	} else {
		logger.Infof("[%s] artefact approved: %s %d (%s)", sessionId, digest, a.Metadata.Size, a.Metadata.Type)
	}

	return nil
}
