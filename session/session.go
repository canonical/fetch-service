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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/canonical/fetch-service/metadata"
)

// Session has information about each authorized client.
type Session struct {
	Id    string
	Pw    string
	Start time.Time
	End   time.Time
	Ctx   *metadata.InspectionContext
	Md    map[string]*metadata.Metadata
}

var (
	makeSessionId = makeSessionIdImpl
	randomString  = randomStringImpl
)

func New() *Session {
	s := &Session{
		Id:    makeSessionId(),
		Pw:    randomString(20),
		Start: time.Now().UTC(),
		Ctx:   metadata.NewInspectionContext(),
		Md:    map[string]*metadata.Metadata{},
	}

	// FIXME: predictable values for testing convenience until the session
	//        creation API is implemented.
	s.Id = "6ba7b8109dad11d180b400c04fd430c8"
	s.Pw = "1ItfzwGBeJ8wsJdP0Nlx"

	log.Printf("creating session %s", s.Id)
	sessions[s.Id] = s

	return s
}

// Finish ends the session and saves metadata.
func (s *Session) Finish() error {
	for k := range s.Md {
		log.Printf("save metadata for artifact %s", k)
		if err := s.SaveMetadata(k); err != nil {
			return err
		}
	}
	s.Discard()
	return nil
}

// Discard deletes this session.
func (s *Session) Discard() {
	_, ok := sessions[s.Id]
	if !ok {
		log.Printf("warning: cannot discard non-existing session %s", s.Id)
		return
	}
	log.Printf("discarding session %s", s.Id)
	delete(sessions, s.Id)
}

// AddMetadata adds downloaded artifact metadata to the current
// session.
func (s *Session) AddMetadata(md *metadata.Metadata) {
	if _, ok := s.Md[md.Sha1]; !ok {
		s.Md[md.Sha1] = md
	}
}

// HasMetadata verifies whether the given digest corresponds
// to an artifact downloaded in this session.
func (s *Session) HasMetadata(sha1 string) bool {
	_, ok := s.Md[sha1]
	return ok
}

// AddDownloadInfo adds the given download information to the
// corresponding artifact metadata.
func (s *Session) AddDownloadInfo(di metadata.DownloadInfo) {
	if s.HasMetadata(di.Sha1) {
		s.Md[di.Sha1].Downloads = append(s.Md[di.Sha1].Downloads, di)
	}
}

// SaveData writes the artifact data correponding to the given
// digest to the asset spool.
func (s *Session) SaveData(digest string) error {
	md, ok := s.Md[digest]
	if !ok {
		return fmt.Errorf("metadata for artifact %s not available", digest)
	}

	dest := filepath.Join(md.AssetDir, fmt.Sprintf("%s.bin", md.Sha1))
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	// Save file data only if it doesn't exist already
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			if err := os.Rename(md.Tempfile, dest); err != nil {
				os.Remove(md.Tempfile)
				return err
			}
		} else {
			os.Remove(md.Tempfile)
			return err
		}
	}

	return nil
}

// SaveMetadata writes the artifact metadata correponsing to the
// given digest to the asset spool.
func (s *Session) SaveMetadata(digest string) error {
	md, ok := s.Md[digest]
	if !ok {
		return fmt.Errorf("metadata for artifact %s not available", digest)
	}

	j, err := json.MarshalIndent(md, "", "\t")
	if err != nil {
		return err
	}

	dest := filepath.Join(md.AssetDir, fmt.Sprintf("%s.json", md.Sha1))
	if err := ioutil.WriteFile(dest, j, 0644); err != nil {
		return err
	}

	return nil
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

// GetSession returns the session corresponding to the given session ID.
func GetSession(id string) *Session {
	s, ok := sessions[id]
	if !ok {
		return nil
	}
	return s
}

// FinishAll gracefully finishes all active sessions.
func FinishAll() {
	for id, s := range sessions {
		log.Printf("finishing session %s", id)
		if err := s.Finish(); err != nil {
			log.Printf("error: %s", err)
		}
	}
	log.Printf("all sessions finished")
}
