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
			case metadata.FileDownload:
				assetDir := v.Md.AssetDir
				sessionId, digest := v.Info.SessionId, v.Md.Sha256

				s := session.GetSession(sessionId)
				if s == nil {
					logger.Warningf("session %s is not active", sessionId)
					break
				}

				// Add download info to artifact metadata
				logger.Infof("[%s] %s %s: %s (%s)", sessionId, v.Info.Method, v.Info.URL, v.Info.Status, v.Info.ContentType)

				if s.HasMetadata(digest) {
					logger.Infof("artifact %s already downloaded", digest)
					s.AddDownloadInfo(v.Info)
					v.Rch <- nil
					break

				}
				s.AddMetadata(&v.Md)
				if err := s.SaveData(digest); err != nil {
					v.Rch <- err
					break
				}

				s.AddDownloadInfo(v.Info)

				go func(md *metadata.Metadata, di *metadata.DownloadInfo, ch chan error) {
					// Extract metadata from file
					if err := s.Insps.Run(assetDir, md, di); err != nil {
						logger.Errorf("%s", err)
						ch <- err
						return
					}

					logger.Infof("[%s] artifact %s %d (%s)", sessionId, digest, md.Size, md.Type)
					ch <- nil
				}(&v.Md, &v.Info, v.Rch)

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
