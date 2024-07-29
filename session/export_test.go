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

package session

import (
	"os"
)

var (
	RandomString = randomString
	Sessions     = sessions
)

func MockMakeSessionId(mock func() string) (restorer func()) {
	old := makeSessionId
	makeSessionId = mock
	return func() {
		makeSessionId = old
	}
}

func MockRandomString(mock func(int) string) (restorer func()) {
	old := randomString
	randomString = mock
	return func() {
		randomString = old
	}
}

func MockGetSession(mock func(string) *Session) (restorer func()) {
	old := GetSession
	GetSession = mock
	return func() {
		GetSession = old
	}
}

func MockOsMkdirAll(mock func(string, os.FileMode) error) (restorer func()) {
	old := osMkdirAll
	osMkdirAll = mock
	return func() {
		osMkdirAll = old
	}
}
