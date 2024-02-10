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

package service

import (
	"math/rand"
	"runtime"
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
	sCount            uint64        // total number of sessions
	sErrors           uint64        // total number of session errors
	sTime             time.Duration // cumulative session time
	sRequests         uint64        // processed requests
	sApprovedRequests uint64        // approved requests
	sRejectedRequests uint64        // rejected requests
	sArtefacts        uint64        // cumulative number of artefacts processed
	sApproved         uint64        // approved artefacts
	sRejected         uint64        // rejected artefacts
	sMaxTime          time.Duration // maximum session duration

}

var proxyNewHttpProxy = proxy.NewHttpProxy

func init() {
	rand.Seed(time.Now().UnixNano())
}

func New(opt *Options) *Service {
	ch := make(chan interface{})
	p := proxyNewHttpProxy(opt.ProxyPort, opt.Spool, ch)
	ctl := control.NewServer(opt.ControlPort, ch)
	start := time.Now()

	return &Service{p: p, ctl: ctl, opt: opt, ch: ch, start: start}
}

// Start runs the fetch service dispatcher.
func (svc *Service) Start() error {
	logger.Info("Starting service...")
	if err := svc.p.Start(); err != nil {
		return err
	}

	//_ = session.New(svc.opt.PermissiveMode) // FIXME: to be created using the API
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
					if svc.sCount > 0 {
						avgTime = time.Duration(uint64(svc.sTime) / svc.sCount)
						avgArts = float32(svc.sArtefacts) / float32(svc.sCount)
						avgReqs = float32(svc.sRequests) / float32(svc.sCount)
					}

					var mem runtime.MemStats
					runtime.ReadMemStats(&mem)

					v.Rch <- messages.ServiceStatus{
						Uptime:                     uint64(time.Since(svc.start).Seconds()),
						StartTime:                  svc.start,
						ActiveSessions:             session.ListAll(),
						SessionCount:               svc.sCount,
						SessionErrors:              svc.sErrors,
						TotalSessionTime:           uint64(svc.sTime.Seconds()),
						ProcessedRequests:          svc.sRequests,
						ApprovedRequests:           svc.sApprovedRequests,
						RejectedRequests:           svc.sRejectedRequests,
						ProcessedArtefacts:         svc.sArtefacts,
						ApprovedArtefacts:          svc.sApproved,
						RejectedArtefacts:          svc.sRejected,
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
						logger.Warningf("session %s is not active", sessionId)
						break
					}

					svc.sRequests++
					s.NumRequests++

					// Run request inspectors
					go func(s *session.Session, a *metadata.Artefact) {
						err := runRequestInspection(s, a)
						if a.Rejected() {
							svc.sRejectedRequests++
							s.RejectedRequests++
						} else {
							svc.sApprovedRequests++
						}
						v.Rch <- err
					}(s, v.A)

				case messages.ResponseInspection:
					sessionId := v.A.SessionId
					digest := v.A.Metadata.Sha256

					s := session.GetSession(sessionId)
					if s == nil {
						logger.Warningf("session %s is not active", sessionId)
						break
					}

					svc.sArtefacts++
					s.NumArtefacts++

					// Add download info to artefact metadata
					dl := v.A.CurrentDownload
					logger.Infof("[%s] %s %s: %s (%s)", sessionId, dl.Method, dl.URL, dl.Status, dl.ContentType)

					if s.HasArtefact(digest) {
						logger.Infof("artefact %s already downloaded", digest)
						s.AddDownload(v.A.CurrentDownload)
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
					go func(s *session.Session, a *metadata.Artefact) {
						err := runResponseInspection(s, a)
						if a.Rejected() {
							svc.sRejected++
							s.RejectedArtefacts++
						} else {
							svc.sApproved++
						}
						v.Rch <- err
					}(s, v.A)

				case messages.CreateSession:
					s := session.New(svc.opt.Spool, svc.opt.PermissiveMode)
					v.Rch <- messages.SessionCredentials{Id: s.Id, Token: s.Pw}
					svc.sCount++

				case messages.ProxyAuth:
					v.Rch <- session.CheckAuth(v.Id, v.Pw)

				case messages.EndSession:
					sessionId := v.Id
					s := session.GetSession(sessionId)
					if s == nil {
						logger.Warningf("session %s is not active", sessionId)
						break
					}

					sm := s.Finish()

					// update stats
					duration := sm.EndTime.Sub(sm.StartTime)
					svc.sTime += duration
					if duration > svc.sMaxTime {
						svc.sMaxTime = duration
					}
					if sm.Err != nil {
						svc.sErrors++
					}

					v.Rch <- messages.SessionResult{
						SessionMetadata: sm,
						Artefacts:       s.Artefacts(),
					}

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
