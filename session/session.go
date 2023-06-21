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
	"encoding/hex"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Session has information about each authorized client.
type Session struct {
	Id    string
	Pw    string
	Start time.Time
	End   time.Time
}

var (
	makeSessionId = makeSessionIdImpl
	randomString  = randomStringImpl
)

func New() *Session {
	s := &Session{
		Id:    makeSessionId(),
		Pw:    randomString(20),
		Start: time.Now(),
	}

	log.Printf("creating session %s", s.Id)
	sessions[s.Id] = s

	return s
}

// Discard ends this session.
func (s *Session) Discard() {
	if !IsActive(s.Id) {
		log.Printf("warning: cannot discard non-existing session %s", s.Id)
		return
	}
	log.Printf("discarding session %s", s.Id)
	delete(sessions, s.Id)
}

// Generate a unique session ID
func makeSessionIdImpl() string {
	id := [16]byte(uuid.New())
	return hex.EncodeToString(id[:])
}

// Generate a random string with the specified length.
func randomStringImpl(length int) string {
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteByte(chars[rand.Intn(len(chars))])
	}
	return b.String()

}

// SessionMap keeps track of all active sessions.
type SessionMap map[string]*Session

var sessions = SessionMap{}

// CheckAuth verifies if the given credentials are valid and match an active session.
func CheckAuth(id string, pw string) bool {
	s, ok := sessions[id]
	if !ok {
		return false
	}
	return s.Pw == pw
}

// IsActive checks whether the given id corresponds to an active session.
func IsActive(id string) bool {
	_, ok := sessions[id]
	return ok
}
