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
	"time"

	"github.com/canonical/fetch-service/control"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service/fetchctl"
	"github.com/canonical/fetch-service/session"
)

var (
	EvaluateRequestInspection     = evaluateRequestInspection
	EvaluateResponseInspection    = evaluateResponseInspection
	LoadHttpProxyRulesOrDefault   = loadHttpProxyRulesOrDefault
	LoadInspectorsConfigOrDefault = loadInspectorsConfigOrDefault
)

func MockNewHttpProxy(mock func(int, string, []byte, []byte, chan interface{}) (*proxy.HttpProxy, error)) (restorer func()) {
	old := proxyNewHttpProxy
	proxyNewHttpProxy = mock
	return func() {
		proxyNewHttpProxy = old
	}
}

func MockNewControlServer(mock func(port int, ch chan interface{}, creds string) *control.Server) (restorer func()) {
	old := controlNewServer
	controlNewServer = mock
	return func() {
		controlNewServer = old
	}
}

func MockNewFetchctlServer(mock func(chan interface{}) *fetchctl.Server) (restorer func()) {
	old := fetchctlNewServer
	fetchctlNewServer = mock
	return func() {
		fetchctlNewServer = old
	}
}

func MockConfigUpdateConfig(mock func(string, bool, []byte, string) error) (restorer func()) {
	old := configUpdateConfig
	configUpdateConfig = mock
	return func() {
		configUpdateConfig = old
	}
}

func MockProxyUpdateCert(mock func(bool, []byte, string, string) error) (restorer func()) {
	old := proxyUpdateCert
	proxyUpdateCert = mock
	return func() {
		proxyUpdateCert = old
	}
}

func MockSessionNewWithId(mock func(string, string, string, time.Duration, bool) *session.Session) (restorer func()) {
	old := sessionNewWithId
	sessionNewWithId = mock
	return func() {
		sessionNewWithId = old
	}
}

func MockConfigLoadProxyHttpRules(mock func(string) error) (restorer func()) {
	old := configLoadHttpProxyRules
	configLoadHttpProxyRules = mock
	return func() {
		configLoadHttpProxyRules = old
	}
}

func MockConfigLoadInspectorsConfig(mock func(string) error) (restorer func()) {
	old := configLoadInspectorsConfig
	configLoadInspectorsConfig = mock
	return func() {
		configLoadInspectorsConfig = old
	}
}
