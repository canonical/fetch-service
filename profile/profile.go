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

package profile

import (
	"fmt"
	"net/http"
	"net/http/pprof"

	"gopkg.in/tomb.v2"

	"github.com/canonical/fetch-service/logger"
)

type Profiler struct {
	port int
	tomb tomb.Tomb
}

func NewProfiler(port int) *Profiler {
	return &Profiler{
		port: port,
	}
}

func (pp *Profiler) Start() {
	addr := fmt.Sprintf(":%d", pp.port)

	logger.Infof("Starting pprof server on %s", addr)

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	pp.tomb.Go(func() error {
		return http.ListenAndServe(addr, mux)
	})
}

func (pp *Profiler) Dying() <-chan struct{} {
	return pp.tomb.Dying()
}

func (pp *Profiler) Err() error {
	return pp.tomb.Err()
}
