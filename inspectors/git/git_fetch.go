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

package git

import (
	"bytes"
	"strings"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/metadata"
)

func GitFetchDetector(raw []byte, limit uint32) bool {
	magic := []byte(" HEAD symref-target:refs/")
	n := len(magic)

	if len(raw) >= 44+n {
		if bytes.Equal(raw[44:44+n], magic) {
			return true
		}
	}

	magic = []byte("000dpackfile\x0a0010\x01PACK")
	n = len(magic)

	if len(raw) >= len(magic) {
		if bytes.Equal(raw[:n], magic) {
			return true
		}
	}

	return false
}

type GitFetchInspector struct {
}

func NewGitFetchInspector() *GitFetchInspector {
	return &GitFetchInspector{}
}

func (GitFetchInspector) ID() string {
	return "git.upload-pack-result"
}

func (ins GitFetchInspector) InspectRequest(a *metadata.Artefact) error {
	dl := a.CurrentDownload
	url := dl.URL

	if dl.Method != "POST" {
		return ErrUnknownRequest
	}

	if !strings.Contains(url, "launchpad.net") && !strings.Contains(url, "github.com") {
		return ErrUnknownRequest
	}

	return nil
}

func (ins *GitFetchInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
	md := a.Metadata

	if md.Type != mimetypes.GitUploadPackResult {
		return nil
	}

	a.Approve(ins, "todo")

	return nil
}
