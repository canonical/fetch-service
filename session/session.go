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
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/canonical/fetch-service/inspectors"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
)

const (
	DefaultSessionTimeout = time.Duration(3 * time.Hour)
)

var (
	ErrInvalidSessionPolicy = errors.New("Invalid session policy")
)

// Session has information about each authorized client.
type Session struct {
	Id         string    // the session ID
	Token      string    // the session token
	Start      time.Time // session start time
	End        time.Time // session end time
	Insps      inspectors.Inspectors
	A          map[metadata.Sha256Digest]*metadata.Artefact
	Permissive bool
	SessionDir string
	Timeout    time.Duration

	// Some stats
	NumRequests       atomic.Uint64
	RejectedRequests  atomic.Uint64
	NumArtefacts      atomic.Uint64
	RejectedArtefacts atomic.Uint64

	revoked bool // session token has been revoked
}

var (
	makeSessionId = makeSessionIdImpl
	randomString  = randomStringImpl
)

func New(spoolDir string, permissive bool) *Session {
	sessionId := makeSessionId()
	s := &Session{
		Id:         sessionId,
		Token:      randomString(20),
		Start:      time.Now().UTC(),
		A:          map[metadata.Sha256Digest]*metadata.Artefact{},
		Permissive: permissive,
		SessionDir: filepath.Join(spoolDir, sessionId),
		Timeout:    DefaultSessionTimeout,
	}

	s.Insps = inspectors.New(permissive)

	var sType string
	if permissive {
		sType = " (permissive)"
	}
	logger.Infof("creating session %s%s", s.Id, sType)

	sessions.Store(s.Id, s)

	return s
}

// Finish ends the session and saves metadata.
func (s *Session) Finish() *metadata.SessionMetadata {
	s.End = time.Now().UTC()

	sm := &metadata.SessionMetadata{
		SessionId:          s.Id,
		StartTime:          s.Start,
		EndTime:            s.End,
		Inspectors:         s.Insps.List(),
		SpoolPath:          s.SessionDir,
		ProcessedRequests:  s.NumRequests.Load(),
		ProcessedArtefacts: s.NumArtefacts.Load(),
		RejectedRequests:   s.RejectedRequests.Load(),
		RejectedArtefacts:  s.RejectedArtefacts.Load(),
		Err:                nil,
	}

	for k := range s.A {
		logger.Infof("save metadata for artefact %s", k)
		if err := s.SaveMetadata(k); err != nil {
			sm.SessionError = err.Error()
			sm.Err = err
			return sm
		}
	}

	if err := s.SaveSessionMetadata(sm); err != nil {
		sm.SessionError = err.Error()
		sm.Err = err
		return sm
	}

	s.Discard()
	return sm
}

// Revoke revokes the session token.
func (s *Session) Revoke() {
	logger.Debugf("[%s] token revoked", s.Id)
	s.revoked = true
}

// IsRevoked returns whether the session token has been revoked.
func (s *Session) IsRevoked() bool {
	return s.revoked
}

// SaveSessionMetadata adds session information to the file spool.
func (s *Session) SaveSessionMetadata(sm *metadata.SessionMetadata) error {

	j, err := json.MarshalIndent(sm, "", "\t")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(s.SessionDir, 0755); err != nil {
		return err
	}

	dest := filepath.Join(s.SessionDir, "session.json")
	if err := os.WriteFile(dest, j, 0644); err != nil {
		return err
	}

	return nil
}

// Discard deletes this session.
func (s *Session) Discard() {
	_, ok := sessions.Load(s.Id)
	if !ok {
		logger.Warningf("cannot discard non-existing session %s", s.Id)
		return
	}
	logger.Infof("discarding session %s", s.Id)

	sessions.Delete(s.Id)
}

func (s *Session) Artefacts() []*metadata.Artefact {
	a := make([]*metadata.Artefact, len(s.A))
	i := 0
	for _, v := range s.A {
		a[i] = v
		i++
	}
	return a
}

// AddArtefact adds downloaded artefact metadata to the current
// session.
func (s *Session) AddArtefact(a *metadata.Artefact) {
	digest := a.Metadata.Sha256
	if _, ok := s.A[digest]; !ok {
		s.A[digest] = a
	}
}

// HasArtefact verifies whether the given digest corresponds
// to an artefact downloaded in this session.
func (s *Session) HasArtefact(sha1 metadata.Sha256Digest) bool {
	_, ok := s.A[sha1]
	return ok
}

// AddDownload adds the given download information to the
// corresponding artefact metadata.
func (s *Session) AddDownload(di metadata.Download) {
	if s.HasArtefact(di.Sha256) {
		s.A[di.Sha256].Downloads = append(s.A[di.Sha256].Downloads, di)
	}
}

// SaveData writes the artefact data correponding to the given
// digest to the asset spool.
func (s *Session) SaveData(digest metadata.Sha256Digest) error {
	a, ok := s.A[digest]
	if !ok {
		return fmt.Errorf("metadata for artefact %s not available", digest)
	}

	dest := filepath.Join(a.AssetDir, fmt.Sprintf("%s.data", a.Metadata.Sha256))
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	// Save file data only if it doesn't exist already
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			if err := os.Rename(a.Tempfile, dest); err != nil {
				os.Remove(a.Tempfile)
				return err
			}
		} else {
			os.Remove(a.Tempfile)
			return err
		}
	}

	return nil
}

// SaveMetadata writes the artefact metadata corresponding to the
// given digest to the asset spool.
func (s *Session) SaveMetadata(digest metadata.Sha256Digest) error {
	a, ok := s.A[digest]
	if !ok {
		return fmt.Errorf("metadata for artefact %s not available", digest)
	}

	j, err := json.MarshalIndent(a, "", "\t")
	if err != nil {
		return err
	}

	dest := filepath.Join(a.AssetDir, fmt.Sprintf("%s.json", a.Metadata.Sha256))
	if err := os.WriteFile(dest, j, 0644); err != nil {
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

type SessionMap struct {
	sync.Map
}

func (smap *SessionMap) Get(id string) *Session {
	s, ok := smap.Load(id)
	if !ok {
		return nil
	}
	return s.(*Session)
}

var sessions = &SessionMap{}

// CheckAuth verifies if the given credentials are valid and match an active session.
func CheckAuth(id string, pw string) bool {
	s := sessions.Get(id)
	if s == nil {
		return false
	}
	if s.revoked {
		return false
	}
	return s.Token == pw
}

// GetSession returns the session corresponding to the given session ID.
var GetSession = GetSessionImpl

func GetSessionImpl(id string) *Session {
	return sessions.Get(id)
}

// FinishAll gracefully finishes all active sessions.
func FinishAll() {
	sessions.Range(func(key, value any) bool {
		id := key.(string)
		s := value.(*Session)
		logger.Infof("finishing session %s", id)
		if sm := s.Finish(); sm.Err != nil {
			logger.Errorf("%s", sm.Err)
		}
		return true
	})
	logger.Info("all sessions finished")
}

// ListAll lists all active session IDs.
func ListAll() []metadata.SessionInfo {
	res := make([]metadata.SessionInfo, 0, 100)
	sessions.Range(func(key, value any) bool {
		id := key.(string)
		s := value.(*Session)

		var policy string
		if s.Permissive {
			policy = "permissive"
		} else {
			policy = "strict"
		}

		res = append(res, metadata.SessionInfo{
			SessionId: id,
			StartTime: s.Start.String(),
			Policy:    policy,
			Age:       uint64(time.Since(s.Start).Seconds()),
			Timeout:   uint64(s.Timeout.Seconds()),
		})
		return true
	})
	return res
}
