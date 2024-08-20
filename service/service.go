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
	"time"

	"gopkg.in/tomb.v2"

	"github.com/canonical/fetch-service/control"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
	"github.com/canonical/fetch-service/version"
)

// Service implements the fetch service main loop.
type Service struct {
	p     *proxy.HttpProxy // proxy instance
	ctl   *control.Server  // control server
	cfg   *config.Server   // configuration server
	ch    chan interface{} // channel to get feedback from handlers
	start time.Time        // service start time (UTC)
	opt   *Options         // configuration options
	tomb  tomb.Tomb        // service dispatcher loop reaper

	totalSessions uint64 // number of created sessions
}

var (
	proxyNewHttpProxy = proxy.NewHttpProxy
	controlNewServer  = control.NewServer
	configNewServer   = config.NewServer
)

func New(opt *Options) (*Service, error) {
	// obtain authentication credentials from the environment
	creds := os.Getenv("FETCH_SERVICE_AUTH")

	ch := make(chan interface{})
	p, err := proxyNewHttpProxy(opt.ProxyPort, opt.Spool, opt.Cert, opt.Key, ch)
	if err != nil {
		return nil, err
	}

	cfg := configNewServer(ch)

	ctl := controlNewServer(opt.ControlPort, ch, creds)
	start := time.Now().UTC()

	return &Service{p: p, ctl: ctl, cfg: cfg, opt: opt, ch: ch, start: start}, nil
}

// Start runs the fetch service dispatcher.
func (svc *Service) Start() error {
	err := config.LoadHttpProxyRules(svc.opt.Config)
	if err != nil {
		return fmt.Errorf("cannot load proxy rules: %s", err)
	}

	logger.Info("Starting service...")

	if err := svc.p.Start(); err != nil {
		return err
	}

	if err := svc.cfg.Start(); err != nil {
		return err
	}

	svc.ctl.Start()

	svc.tomb.Go(svc.dispatcher)

	return nil
}

var configUpdateConfig = config.UpdateConfig

func (svc *Service) dispatcher() error {
	// Set up idle auto-shutdown
	idleTimer := time.NewTimer(time.Duration(svc.opt.IdleShutdown) * time.Second)
	if svc.opt.IdleShutdown == 0 {
		if !idleTimer.Stop() {
			<-idleTimer.C
		}
	}

loop:
	for {
		select {
		case msg := <-svc.ch:
			if svc.opt.IdleShutdown > 0 {
				idleTimer.Reset(time.Duration(svc.opt.IdleShutdown) * time.Second)
			}
			logger.Infof("[service] received message: %T", msg)

			switch v := msg.(type) {
			case messages.GetServiceStatus:
				v.Rch <- messages.ServiceStatus{
					Uptime:         uint64(time.Since(svc.start).Seconds()),
					StartTime:      svc.start,
					SessionCount:   svc.totalSessions,
					ActiveSessions: session.SessionInfos(),
				}

			case messages.RequestInspection:
				sessionId := v.A.SessionId

				s := session.GetSession(sessionId)
				if s == nil {
					v.Rch <- fmt.Errorf("cannot inspect request: session %s is not active", sessionId)
					break
				}

				// Run request inspectors
				go func(s *session.Session, a *metadata.Artefact) {
					v.Rch <- runRequestInspection(s, a)
				}(s, v.A)

			case messages.ResponseInspection:
				sessionId := v.A.SessionId
				digest := v.A.Metadata.Sha256

				s := session.GetSession(sessionId)
				if s == nil {
					v.Rch <- fmt.Errorf("cannot inspect response: session %s is not active", sessionId)
					break
				}

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
				go func(s *session.Session, a *metadata.Artefact) {
					v.Rch <- runResponseInspection(s, a)
				}(s, v.A)

			case messages.CreateSession:
				permissive := false
				if v.Policy == "permissive" {
					if svc.opt.PermissiveMode {
						permissive = true
					} else {
						v.Rch <- messages.SessionCredentials{
							Err: session.ErrInvalidSessionPolicy,
						}
						break
					}
				}

				timeout := time.Duration(v.Timeout * uint64(time.Second))
				s := session.New(svc.opt.Spool, timeout, permissive)
				svc.totalSessions++
				v.Rch <- messages.SessionCredentials{Id: s.Id, Token: s.Token}

			case messages.RevokeToken:
				sessionId := v.Id
				s := session.GetSession(sessionId)
				if s == nil {
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

				v.Rch <- messages.RevokeTokenResult{
					SessionId: s.Id,
					StartTime: s.Start.String(),
					EndTime:   s.End.String(),
					SpoolPath: svc.opt.Spool,
				}

			case messages.SessionReport:
				sessionId := v.Id
				s := session.GetSession(sessionId)
				if s == nil {
					v.Rch <- messages.SessionReportResult{
						SessionMetadata: &metadata.SessionMetadata{Err: messages.ErrSessionNotFound},
						Artefacts:       []*metadata.Artefact{},
						Err:             messages.ErrSessionNotFound,
					}
					break
				}

				if !s.IsRevoked() {
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

			case messages.Configuration:
				logger.Infof("[service] configuration operation: %s", v.Operation)
				var reply messages.ConfigurationResult
				switch v.Operation {
				case "version":
					reply = messages.ConfigurationResult{
						Status:  "ok",
						Message: version.Version,
					}
				case "update-config":
					err := configUpdateConfig(v.Type, v.ValidateOnly, v.Payload, svc.opt.Config)
					if err != nil {
						reply = messages.ConfigurationResult{
							Status:  "error",
							Message: fmt.Sprintf("%s configuration update error", v.Type),
						}
						logger.Warningf("[service] %s update error: %s", v.Type, err.Error())
					} else if v.ValidateOnly {
						reply = messages.ConfigurationResult{
							Status:  "ok",
							Message: "configuration validated",
						}
						logger.Infof("[service] %s configuration validated", v.Type)
					} else {
						reply = messages.ConfigurationResult{
							Status:  "ok",
							Message: "configuration updated",
						}
						logger.Infof("[service] %s configuration updated", v.Type)
					}

				default:
					reply = messages.ConfigurationResult{
						Status:  "error",
						Message: "unsupported operation",
					}
				}
				v.Rch <- reply

			default:
				logger.Warningf("[service] unknown message type %T", v)
			}

		case sessionId := <-session.ExpiredSessionId:
			logger.Infof("[%s] session expired", sessionId)
			s := session.GetSession(sessionId)
			if s == nil {
				logger.Warningf("[service] session %s does not exist", sessionId)
				break
			}
			if err := s.Finish(); err != nil {
				logger.Errorf("[%s] cannot finish session: %s", sessionId, err)
			}
			if err := session.RemoveResources(svc.opt.Spool, sessionId); err != nil {
				logger.Errorf("[%s] cannot remove session resources: %s", sessionId, err)
			}
			if svc.opt.IdleShutdown > 0 {
				idleTimer.Reset(time.Duration(svc.opt.IdleShutdown) * time.Second)
			}

		case <-svc.tomb.Dying():
			break loop

		case <-svc.ctl.Dying():
			break loop

		case <-svc.cfg.Dying():
			break loop

		case <-svc.p.Dying():
			break loop

		case <-idleTimer.C:
			n := session.NumSessions()
			if n < 1 {
				logger.Infof("auto-shutdown after being idle for %d seconds", svc.opt.IdleShutdown)
				break loop
			} else {
				logger.Infof("number of active sessions: %d", n)
			}
		}
	}

	return nil
}

func (svc *Service) Stop() error {
	logger.Info("Stopping service...")
	session.FinishAll()

	if err := svc.p.Stop(); err != nil {
		logger.Warningf("Cannot shut down the HTTP server: %s", err)
		return err
	}

	if err := svc.cfg.Stop(); err != nil {
		logger.Warningf("Cannot shut down the configuration socket: %s", err)
		return err
	}

	if err := svc.ctl.Stop(); err != nil {
		logger.Warningf("Cannot shut down the control API server: %s", err)
		return err
	}

	svc.tomb.Kill(nil)
	if err := svc.tomb.Wait(); err != nil {
		return err
	}

	return nil
}

func (svc *Service) Alive() bool {
	return svc.tomb.Alive()
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

	if a.RequestRejected() {
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
		a.Result = opinions.Rejected
		return err
	}

	sessionId := s.Id
	digest := a.Metadata.Sha256

	if a.Rejected() {
		a.Result = opinions.Rejected
		if s.Permissive {
			logger.Infof("[%s] artefact %s %d (%s) would be rejected (permissive)",
				sessionId, digest, a.Metadata.Size, a.Metadata.Type)
		} else {
			logger.Infof("[%s] artefact rejected: %s %d (%s)",
				sessionId, digest, a.Metadata.Size, a.Metadata.Type)
			return ErrRejectedArtefact
		}
	} else {
		a.Result = opinions.Approved
		logger.Infof("[%s] artefact approved: %s %d (%s)", sessionId, digest, a.Metadata.Size, a.Metadata.Type)
	}

	return nil
}
