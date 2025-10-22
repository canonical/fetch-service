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

package session

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/canonical/fetch-service/secrets"
	"github.com/google/uuid"

	"github.com/canonical/fetch-service/inspectors"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/utils"
	"github.com/canonical/fetch-service/version"
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
	Id            string    // the session ID
	Token         string    // the session token
	Start         time.Time // session start time
	End           time.Time // session end time
	Insps         inspectors.Inspectors
	A             map[digests.Sha256Digest]*metadata.Artifact
	Permissive    bool          // whether this is a permissive session
	SessionDir    string        // the session path including spool
	CacheDir      string        // location of session-specific cache
	Timeout       time.Duration // maximum time allowed for a session
	InspectorsCfg config.InspectorsConfig
	Logger        logger.Logger    // Session-aware log helper
	Secrets       []secrets.Secret // Per-session secrets

	timer   *sessionTimer // timeout to auto-finish an idle session
	revoked bool          // session token has been revoked
}

var (
	makeSessionId = makeSessionIdImpl
	randomString  = randomStringImpl
)

// New creates a session that stores artifact data and metadata under
// spoolDir. The session is automatically finished if it times out.
func New(spoolDir string, timeout time.Duration, permissive bool, secrets []secrets.Secret) *Session {
	sessionId := makeSessionId()
	token := randomString(20)

	return NewWithId(sessionId, token, spoolDir, timeout, permissive, secrets)
}

// NewWithId creates a session using the specified sessionId and token.
func NewWithId(sessionId, token, spoolDir string, timeout time.Duration, permissive bool, secrets []secrets.Secret) *Session {
	_, ok := sessions.Load(sessionId)
	if ok {
		id := makeSessionId()
		logger.Warningf("cannot recreate existing session ID %s, use %s instead", sessionId, id)
		sessionId = id
	}

	if timeout == 0 {
		timeout = DefaultSessionTimeout
	}

	s := &Session{
		Id:            sessionId,
		Token:         token,
		Start:         time.Now().UTC(),
		A:             map[digests.Sha256Digest]*metadata.Artifact{},
		Permissive:    permissive,
		SessionDir:    filepath.Join(spoolDir, sessionId),
		CacheDir:      filepath.Join(spoolDir, sessionId, "cache"),
		Timeout:       timeout,
		InspectorsCfg: config.GetInspectorsConfig(),
		Logger:        logger.NewSessionLogger(sessionId),
		Secrets:       secrets,
	}

	cfg := config.GetInspectorsConfig()
	s.Insps = inspectors.New(permissive, cfg)

	var sType = "strict"
	if permissive {
		sType = "permissive"
	}
	s.Logger.Infof("create %s session, timeout = %s", sType, timeout)

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
		Generator:  fmt.Sprintf("fetch-service %s", version.Version),
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
		s.Logger.Infof("save metadata for artifact %s", k)
		if err := s.SaveMetadata(k); err != nil {
			return err
		}
	}

	if err := s.SaveSessionMetadata(sm); err != nil {
		return err
	}

	// cleanup cache dir, if it exists, as its contents can no longer be used
	if stat, err := os.Stat(s.CacheDir); err == nil && stat.IsDir() {
		if removeErr := os.RemoveAll(s.CacheDir); removeErr != nil {
			return removeErr
		}
	}

	s.Discard()

	return nil

}

// Revoke revokes the session token.
func (s *Session) Revoke(token string) bool {
	if token != s.Token {
		return false
	}
	s.Logger.Debug("token revoked")
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
		s.Logger.Warning("cannot discard non-existing session")
		return
	}
	s.Logger.Info("discarding session")
	sessions.Delete(s.Id)
	s.timer.Cancel() // end session timer
	s.Logger.Info("session discarded")
}

func (s *Session) Artifacts() []*metadata.Artifact {
	a := make([]*metadata.Artifact, len(s.A))
	i := 0
	for _, v := range s.A {
		a[i] = v
		i++
	}
	return a
}

// AddArtifact adds downloaded artifact metadata to the current
// session.
func (s *Session) AddArtifact(a *metadata.Artifact) {
	digest := a.Metadata.Sha256
	if _, ok := s.A[digest]; !ok {
		s.A[digest] = a
	}
}

// HasArtifact verifies whether the given digest corresponds
// to an artifact downloaded in this session.
func (s *Session) HasArtifact(sha256 digests.Sha256Digest) bool {
	_, ok := s.A[sha256]
	return ok
}

// ArtifactResult obtains the result from a previous HasArtifact
// inspection, or Rejected if it was not previously inspected.
func (s *Session) ArtifactResult(sha256 digests.Sha256Digest) opinions.OpinionKind {
	a, ok := s.A[sha256]
	if !ok {
		return opinions.Rejected
	}
	return a.Result
}

// AddDownload adds the given download information to the
// corresponding artifact metadata.
func (s *Session) AddDownload(di metadata.Download) {
	if s.HasArtifact(di.Sha256) {
		s.A[di.Sha256].Downloads = append(s.A[di.Sha256].Downloads, di)
	}
}

var osMkdirAll = os.MkdirAll

// SaveData moves the artifact file to the asset spool.
func (s *Session) SaveData(a *metadata.Artifact) error {
	defer s.removeTempFile(a.Tempfile)

	dest := filepath.Join(a.AssetDir, fmt.Sprintf("%s.data", a.Metadata.Sha256))
	if err := osMkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	// Save file data only if it doesn't exist already
	if _, err := os.Stat(dest); err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		// Move temporary file to spool
		if err := utils.MoveFile(a.Tempfile, dest); err != nil {
			return err
		}
	}

	return nil
}

// SaveMetadata writes the artifact metadata corresponding to the
// given digest to the asset spool.
func (s *Session) SaveMetadata(digest digests.Sha256Digest) error {
	a, ok := s.A[digest]
	if !ok {
		return fmt.Errorf("metadata for artifact %s not available", digest)
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

func (s *Session) removeTempFile(name string) {
	if err := os.Remove(name); err != nil {
		if !os.IsNotExist(err) {
			s.Logger.Warningf("cannot remove temporary file %s: %s", name, err)
		}
	}
}

// Generate a unique session ID
func makeSessionIdImpl() string {
	id := [16]byte(uuid.New())
	return hex.EncodeToString(id[:])
}

// Generate a random string with the specified length.
func randomStringImpl(length int) string {
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := 0; i < length; i++ {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)

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
		s := value.(*Session)
		s.Logger.Info("finishing session")
		if err := s.Finish(); err != nil {
			s.Logger.Errorf("cannot finish session: %s", err.Error())
		}
		return true
	})
	logger.Info("all sessions finished")
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
