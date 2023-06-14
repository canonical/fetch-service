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

	"github.com/canonical/fetch-service/proxy"
)

type Service struct {
	p   *proxy.HttpProxy // proxy instance
	ch  chan interface{} // channel to get feedback from handlers
	opt *Options         // configuration options
}

var proxyNewHttpProxy = proxy.NewHttpProxy

func New(opt *Options) *Service {

	ch := make(chan interface{})
	p := proxyNewHttpProxy(opt.Port, ch)

	return &Service{p: p, opt: opt, ch: ch}
}

func (s *Service) Start() {
	log.Printf("Starting service...")
	s.p.Start()

	for {
		select {
		case msg := <-s.ch:
			switch v := msg.(type) {
			case proxy.DownloadInfo:
				log.Printf("%s %s: %s (%s)", v.Method, v.URL, v.Status, v.ContentType)
			default:
				log.Printf("Unknown message type %T", v)
			}
		}
	}
}
