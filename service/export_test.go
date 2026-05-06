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
	"github.com/canonical/fetch-service/secrets"
	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/service/fetchctl"
	"github.com/canonical/fetch-service/session"
)

var (
	EvaluateRequestInspection          = evaluateRequestInspection
	EvaluateResponseInspection         = evaluateResponseInspection
	LoadHTTPProxyRulesOrDefault        = loadHTTPProxyRulesOrDefault
	LoadDefaultInspectorsConfigCombine = loadDefaultInspectorsConfigCombine

	HandleResponseInspection = handleResponseInspection
	HandleCompleteInspection = handleCompleteInspection
)

func MockNewHTTPProxy(mock func(int, string, []byte, []byte, chan interface{}) (*proxy.HTTPProxy, error)) (restorer func()) {
	old := proxyNewHTTPProxy
	proxyNewHTTPProxy = mock
	return func() {
		proxyNewHTTPProxy = old
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

func MockFetchctlServerStart(mock func(*fetchctl.Server) error) (restorer func()) {
	old := fetchctlServerStart
	fetchctlServerStart = mock
	return func() {
		fetchctlServerStart = old
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

func MockSessionNewWithID(mock func(string, string, string, time.Duration, bool, []secrets.Secret, config.OverrideInspectorsConfig) *session.Session) (restorer func()) {
	old := sessionNewWithID
	sessionNewWithID = mock
	return func() {
		sessionNewWithID = old
	}
}

func MockConfigLoadProxyHTTPRules(mock func(string) error) (restorer func()) {
	old := configLoadHTTPProxyRules
	configLoadHTTPProxyRules = mock
	return func() {
		configLoadHTTPProxyRules = old
	}
}

func MockConfigLoadInspectorsConfig(mock func(string) error) (restorer func()) {
	old := configLoadInspectorsConfig
	configLoadInspectorsConfig = mock
	return func() {
		configLoadInspectorsConfig = old
	}
}

func MockConfigLoadOverrideInspectorsConfig(mock func(string) error) (restorer func()) {
	old := configLoadOverrideInspectorsConfig
	configLoadOverrideInspectorsConfig = mock
	return func() {
		configLoadOverrideInspectorsConfig = old
	}
}
