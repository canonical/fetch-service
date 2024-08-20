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
	"github.com/canonical/fetch-service/control"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service/config"
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

func MockNewConfigServer(mock func(chan interface{}) *config.Server) (restorer func()) {
	old := configNewServer
	configNewServer = mock
	return func() {
		configNewServer = old
	}
}

func MockConfigUpdateConfig(mock func(string, bool, []byte, string) error) (restorer func()) {
	old := configUpdateConfig
	configUpdateConfig = mock
	return func() {
		configUpdateConfig = old
	}
}
