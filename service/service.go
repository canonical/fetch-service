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
	"log"
	"math/rand"
	"time"

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
func (s *Service) Start() {
	log.Printf("Starting service...")
	s.p.Start()

	_ = session.New() // XXX: to be created using the API

	for {
		select {
		case msg := <-s.ch:
			switch v := msg.(type) {
			case metadata.DownloadInfo:
				log.Printf("[%s] %s %s: %s (%s)", v.SessionId, v.Method, v.URL, v.Status, v.ContentType)
			case proxy.ProxyAuth:
				v.Rch <- session.CheckAuth(v.Id, v.Pw)
			default:
				log.Printf("Unknown message type %T", v)
			}
		}
	}
}
