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
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
)

// Service implements the fetch service main loop.
type Service struct {
	p   *proxy.HttpProxy // proxy instance
	ch  chan interface{} // channel to get feedback from handlers
	opt *Options         // configuration options
}

var proxyNewHttpProxy = proxy.NewHttpProxy

func init() {
	rand.Seed(time.Now().UnixNano())
}

func New(opt *Options) *Service {
	ch := make(chan interface{})
	p := proxyNewHttpProxy(opt.Port, opt.Spool, ch)

	return &Service{p: p, opt: opt, ch: ch}
}

// Start runs the fetch service dispatcher.
func (svc *Service) Start() {
	logger.Info("Starting service...")
	svc.p.Start()

	_ = session.New() // FIXME: to be created using the API
	defer session.FinishAll()

	// Shut down gracefully if terminated.
	cs := make(chan os.Signal, 1)
	signal.Notify(cs, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-cs
		svc.ch <- sig
	}()

dispatcherLoop:
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

			case messages.ArtefactDownload:
				assetDir := v.A.AssetDir
				sessionId := v.A.SessionId
				digest := v.A.Metadata.Sha256

				s := session.GetSession(sessionId)
				if s == nil {
					logger.Warningf("session %s is not active", sessionId)
					break
				}

				// Add download info to artifact metadata
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

					logger.Infof("[%s] artifact %s %d (%s)", sessionId, digest, a.Metadata.Size, a.Metadata.Type)
					rch <- nil
				}(v.A, v.Rch)

			case proxy.ProxyAuth:
				v.Rch <- session.CheckAuth(v.Id, v.Pw)

			case os.Signal:
				if v == syscall.SIGINT {
					break dispatcherLoop
				}

			default:
				logger.Warningf("Unknown message type %T", v)
			}
		}
	}
}
