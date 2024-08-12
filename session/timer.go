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

package session

import (
	"time"
)

// sessionTimer manages the session timeout timer.
type sessionTimer struct {
	*time.Timer               // the session timer
	done        chan struct{} // channel to signal monitoring end
}

// newSessionTimer creates a session timer with duration d, and executes
// function f on expiration. If the timer is cancelled, it will never
// expire and function g is executed instead.
func newSessionTimer(s *Session, ch chan string) *sessionTimer {
	t := &sessionTimer{
		Timer: time.NewTimer(s.Timeout),
		done:  make(chan struct{}, 1),
	}

	go func() {
		select {
		case <-t.C:
			ch <- s.Id
		case <-t.done:
		}
	}()

	return t
}

// Cancel stops time monitoring.
func (t *sessionTimer) Cancel() {
	t.Stop()
	t.done <- struct{}{}
}
