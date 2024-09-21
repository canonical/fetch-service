// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2024 Canonical Ltd.
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
	"time"

	"github.com/google/uuid"

	"github.com/canonical/fetch-service/inspectors"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/utils"
)

const (
	DefaultSessionTimeout = time.Duration(6 * time.Hour)
)

var (
	ErrInvalidSessionPolicy = errors.New("Invalid session policy")

	ExpiredSessionId = make(chan string, 1)
)

// Session has information about each authorized client.
type Session struct {
	Id         string    // the session ID
	Token      string    // the session token
	Start      time.Time // session start time
	End        time.Time // session end time
	Insps      inspectors.Inspectors
	A          map[digests.Sha256Digest]*metadata.Artefact
	Permissive bool          // whether this is a permissive session
	SessionDir string        // the session path including spool
	Timeout    time.Duration // maximum time allowed for a session

	timer   *sessionTimer // auto-finish the session after a Timeout
	revoked bool          // session token has been revoked
}

var (
	makeSessionId = makeSessionIdImpl
	randomString  = randomStringImpl
)

// New creates a session that stores artefact data and metadata under
// spoolDir. The session is automatically finished if it times out.
func New(spoolDir string, timeout time.Duration, permissive bool) *Session {
	sessionId := makeSessionId()
	token := randomString(20)

	return NewWithId(sessionId, token, spoolDir, timeout, permissive)
}

// NewWithId creates a session using the specified sessionId and token.
func NewWithId(sessionId, token, spoolDir string, timeout time.Duration, permissive bool) *Session {
	if timeout == 0 {
		timeout = DefaultSessionTimeout
	}

	s := &Session{
		Id:         sessionId,
		Token:      token,
		Start:      time.Now().UTC(),
		A:          map[digests.Sha256Digest]*metadata.Artefact{},
		Permissive: permissive,
		SessionDir: filepath.Join(spoolDir, sessionId),
		Timeout:    timeout,
	}

	s.Insps = inspectors.New(permissive)

	var sType string
	if permissive {
		sType = " (permissive)"
	}
	logger.Infof("[%s] creating session%s, timeout = %s", s.Id, sType, timeout)

	sessions.Store(s.Id, s)
	s.timer = newSessionTimer(s, ExpiredSessionId)

	return s
}

func (s *Session) Metadata() *metadata.SessionMetadata {
	var policy string
	if s.Permissive {
		policy = "permissive"
	} else {
		policy = "strict"
	}

	return &metadata.SessionMetadata{
		Policy:     policy,
		Comment:    "Metadata format is unstable and may change without prior notice.",
		SessionId:  s.Id,
		StartTime:  s.Start,
		EndTime:    s.End,
		Inspectors: s.Insps.List(),
		SpoolPath:  s.SessionDir,
		Err:        nil,
	}
}

// Finish ends the session and saves metadata.
func (s *Session) Finish() error {
	s.timer.Stop()

	sm := s.Metadata()

	for k := range s.A {
		logger.Infof("[%s] save metadata for artefact %s", s.Id, k)
		if err := s.SaveMetadata(k); err != nil {
			return err
		}
	}

	if err := s.SaveSessionMetadata(sm); err != nil {
		return err
	}

	s.Discard()

	return nil

}

// Revoke revokes the session token.
func (s *Session) Revoke(token string) bool {
	if token != s.Token {
		return false
	}
	logger.Debugf("[%s] token revoked", s.Id)
	s.revoked = true
	s.End = time.Now().UTC()
	return true
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

	if err := osMkdirAll(s.SessionDir, 0755); err != nil {
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
		logger.Warningf("[%s] cannot discard non-existing session", s.Id)
		return
	}
	logger.Infof("[%s] discarding session", s.Id)
	sessions.Delete(s.Id)
	s.timer.Cancel() // end session timer
	logger.Infof("[%s] session discarded", s.Id)
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
func (s *Session) HasArtefact(sha1 digests.Sha256Digest) bool {
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

var osMkdirAll = os.MkdirAll

// SaveData writes the artefact data corresponding to the given
// digest to the asset spool.
func (s *Session) SaveData(digest digests.Sha256Digest) error {
	a, ok := s.A[digest]
	if !ok {
		return fmt.Errorf("metadata for artefact %s not available", digest)
	}

	dest := filepath.Join(a.AssetDir, fmt.Sprintf("%s.data", a.Metadata.Sha256))
	if err := osMkdirAll(filepath.Dir(dest), 0755); err != nil {
		os.Remove(a.Tempfile)
		return err
	}

	// Save file data only if it doesn't exist already
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			// Move temporary file to spool
			if err := utils.MoveFile(a.Tempfile, dest); err != nil {
				os.Remove(a.Tempfile)
				return err
			}
		} else {
			os.Remove(a.Tempfile)
			return err
		}
	} else {
		// Remove temporary file if it already exists
		os.Remove(a.Tempfile)
	}

	return nil
}

// SaveMetadata writes the artefact metadata corresponding to the
// given digest to the asset spool.
func (s *Session) SaveMetadata(digest digests.Sha256Digest) error {
	a, ok := s.A[digest]
	if !ok {
		return fmt.Errorf("metadata for artefact %s not available", digest)
	}

	j, err := utils.JSONMarshalIndentNoHTMLEscape(a, "", "\t")
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

func (sm *SessionMap) Get(id string) *Session {
	s, ok := sm.Load(id)
	if !ok {
		return nil
	}
	return s.(*Session)
}

func (sm *SessionMap) Size() int {
	size := 0
	sm.Range(func(key, value interface{}) bool {
		size++
		return true
	})
	return size
}

var (
	sessions = &SessionMap{}
)

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
		logger.Infof("[session] finishing session %s", id)
		if err := s.Finish(); err != nil {
			logger.Errorf("[session] cannot finish session: %s", err)
		}
		return true
	})
	logger.Info("[session] all sessions finished")
}

// NumSessions returns the number of active sessions.
func NumSessions() int {
	return sessions.Size()
}

// SessionInfos returns the list of infos for all active sessions.
func SessionInfos() []metadata.SessionInfo {
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
