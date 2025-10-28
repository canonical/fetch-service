// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024 Canonical Ltd.
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

package session_test

import (
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/session"
)

type sessionTimerSuite struct{}

var _ = Suite(&sessionTimerSuite{})

func (t *sessionTimerSuite) TestExpiredSessionTimer(c *C) {
	s := session.New("", 500*time.Millisecond, true, nil, config.SessionInspectorsConfig{})
	defer s.Discard()

	ch := make(chan string, 1)
	_ = session.NewSessionTimer(s, ch)

	expired := <-ch
	c.Assert(expired, Equals, s.ID)
}

func (t *sessionTimerSuite) TestCanceledSessionTimer(c *C) {
	s := session.New("", 2*time.Second, true, nil, config.SessionInspectorsConfig{})
	defer s.Discard()

	ch := make(chan string, 1)

	timer := session.NewSessionTimer(s, ch)
	time.Sleep(1 * time.Second)
	timer.Cancel()

	select {
	case <-ch:
		c.Fail()
	case <-time.NewTimer(2 * time.Second).C:
		c.Succeed()
	}
}
