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
	"time"

	"gopkg.in/tomb.v2"

	"github.com/canonical/fetch-service/control"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
)

// Service implements the fetch service main loop.
type Service struct {
	p      *proxy.HttpProxy // proxy instance
	ctl    *control.Server  // control server
	ch     chan interface{} // channel to get feedback from handlers
	start  time.Time        // service start time (UTC)
	opt    *Options         // configuration options
	tomb   tomb.Tomb        // service dispacher loop reaper
	sCount int              // total number of sessions
}

var proxyNewHttpProxy = proxy.NewHttpProxy

func init() {
	rand.Seed(time.Now().UnixNano())
}

func New(opt *Options) *Service {
	ch := make(chan interface{})
	p := proxyNewHttpProxy(opt.Port, opt.Spool, ch)
	ctl := control.NewServer(9999, ch)
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
				case messages.RequestAuthorization:
					sessionId := v.A.SessionId
					s := session.GetSession(sessionId)
					if s == nil {
						logger.Warningf("session %s is not active", sessionId)
						break
					}

					go func(a *metadata.Artefact, rch chan error) {
						// Check request
						if err := s.Insps.RunRequestInspectors(a); err != nil {
							logger.Errorf("%s", err)
							rch <- err
							return
						}

						dl := v.A.CurrentDownload
						logger.Infof("[%s] %s %s: request approved", sessionId, dl.Method, dl.URL)
						rch <- nil
					}(v.A, v.Rch)

				case messages.GetServiceStatus:
					v.Rch <- messages.ServiceStatus{
						StartTime:      svc.start,
						ActiveSessions: session.ListAll(),
						SessionCount:   svc.sCount,
					}

				case messages.ArtefactDownload:
					assetDir := v.A.AssetDir
					sessionId := v.A.SessionId
					digest := v.A.Metadata.Sha256

					s := session.GetSession(sessionId)
					if s == nil {
						logger.Warningf("session %s is not active", sessionId)
						break
					}

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

					go func(a *metadata.Artefact, rch chan error) {
						// Extract metadata from file
						if err := s.Insps.RunArtefactInspectors(assetDir, a); err != nil {
							logger.Errorf("%s", err)
							rch <- err
							return
						}

						logger.Infof("[%s] artefact %s %d (%s)", sessionId, digest, a.Metadata.Size, a.Metadata.Type)
						rch <- nil
					}(v.A, v.Rch)

				case messages.CreateSession:
					s := session.New(svc.opt.Spool, svc.opt.PermissiveMode)
					v.Rch <- messages.SessionCredentials{Id: s.Id, Pw: s.Pw}
					svc.sCount++

				case messages.ProxyAuth:
					v.Rch <- session.CheckAuth(v.Id, v.Pw)

				case messages.EndSession:
					sessionId := v.Id
					s := session.GetSession(sessionId)
					var res messages.SessionResult

					sm, err := s.Finish()
					if err != nil {
						res = messages.SessionResult{Err: err}
					} else {
						res = messages.SessionResult{SessionMetadata: sm}
						res.Artefacts = s.Artefacts()
					}
					v.Rch <- res

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
