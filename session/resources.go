// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
)

func SessionMetadataWritten(spoolDir, sessionID string) bool {
	// Check if metadata path exists
	metadataPath := filepath.Join(spoolDir, sessionID, "session.json")
	if _, err := os.Stat(metadataPath); err != nil {
		return false
	}
	return true
}

func LoadSessionMetadata(spoolDir, sessionID string) (*metadata.SessionMetadata, error) {
	logger.Infof("load session %s metadata", sessionID)

	metadataPath := filepath.Join(spoolDir, sessionID, "session.json")

	f, err := os.Open(metadataPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	decoder := json.NewDecoder(f)
	var sm metadata.SessionMetadata

	if err := decoder.Decode(&sm); err != nil {
		return nil, err
	}

	return &sm, nil
}

func RemoveResources(spoolDir, sessionID string) error {
	sessionDir := filepath.Join(spoolDir, sessionID)
	logger.Infof("removing session %s resources", sessionID)
	if err := os.RemoveAll(sessionDir); err != nil {
		return err
	}
	return nil
}
