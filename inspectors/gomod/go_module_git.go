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

package gomod

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/git"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/utils"
)

const (
	GitUploadPackID = "git.upload-pack"
)

// The GoModuleGitInspector handles upload-pack requests. It recognizes
// the "ls-refs" and "fetch" commands.
type GoModuleGitInspector struct {
}

func NewGoModuleGitInspector() *GoModuleGitInspector {
	return &GoModuleGitInspector{}
}

func (GoModuleGitInspector) ID() string {
	return "go.module.git"
}

// InspectRequest verifies whether this is a valid upload-pack request. For
// it to succeed the following conditions must be satisfied:
//
//   - The "Git-Protocol" request header must be set to "version=2".
//   - The Content-Type header must be set to "application/x-git-upload-pack-request".
//   - The Accept header must be set to "application/x-git-upload-pack-result"
//   - The request URL must match a valid upload-pack pattern.
//   - The upload-pack command must be "ls-refs" or "fetch".
//   - If command is "fetch", it must want a single shallow ref.
func (ins *GoModuleGitInspector) InspectRequest(a RequestArtefact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	content_type, ok := a.RequestHeader("Content-Type")
	if !ok || len(content_type) < 1 || content_type[0] != "application/x-git-upload-pack-request" {
		return nil // we don't recognize this request
	}

	accept, ok := a.RequestHeader("Accept")
	if !ok || len(accept) < 1 || accept[0] != "application/x-git-upload-pack-result" {
		return nil // we don't recognize this request
	}

	_, err = newGoModuleGitUrlInfo(u)
	if err != nil {
		return nil // we don't recognize this request
	}

	command, ok := a.RequestStringAnnotation(GitUploadPackID, "command")
	if !ok || command != "fetch" {
		return nil // we don't recognize this request
	}

	a.SetRequestPending(ins, "valid URL for go module download")
	return nil
}

// InspectArtefact verifies if the fetched repository data
// contains a go module.
func (ins *GoModuleGitInspector) InspectArtefact(f ArtefactReader, a ResponseArtefact) error {

	if a.ContentType() != "application/x-git-upload-pack-result" {
		return nil
	}

	command, ok := a.RequestStringAnnotation(GitUploadPackID, "command") // the upload-pack request command
	if !ok {
		// this must have been set by the git upload-pack inspector
		a.SetResponseUnknown(ins, "command not set during request inspection")
		return nil
	}
	notes := Annotation{}

	logger.Debugf("inspect git upload-pack artefact: command %q", command)

	// We're only interested in the fetch command
	if command != "fetch" {
		return nil
	}

	if a.MimetypeIs("text/plain") {
		return nil
	}

	if !a.MimetypeIs("application/octet-stream") {
		a.SetResponseRejected(ins, "bad data type for go module")
		return nil
	}

	// Read wants information from the git inspector annotation
	w, has_wants := a.RequestAnnotation(GitUploadPackID, "wants")
	wr, has_want_refs := a.RequestAnnotation(GitUploadPackID, "want-refs")
	if !has_wants && !has_want_refs {
		// this must have been set by the git upload-pack inspector
		return errors.New("cannot read request want/want-ref annotation")
	}

	var wants []string
	if has_wants {
		var ok bool
		wants, ok = w.([]string)
		if !ok || len(wants) < 1 {
			return errors.New("cannot read want annotation")
		}
	}

	var want_refs []string
	if has_want_refs {
		var ok bool
		want_refs, ok = wr.([]string)
		if !ok || len(want_refs) < 1 {
			return errors.New("cannot read want-ref annotation")
		}
	}

	// Read depth information from the git inspector annotation
	isShallow, ok := a.RequestBoolAnnotation(GitUploadPackID, "is-shallow")
	if !ok {
		return errors.New("cannot read is-shallow annotation")
	}

	// Unpack and checkout in temporary directory
	dir, err := os.MkdirTemp("", "fetch-")
	if err != nil {
		return err
	}
	logger.Debugf("unpack objects in %s", dir)

	defer os.RemoveAll(dir)

	// Unshallow is unsupported
	unshallow, ok := a.ResponseBoolAnnotation(GitUploadPackID, "unshallow")
	if ok && unshallow {
		a.SetResponseUnknown(ins, "unshallow is not supported").Annotate(notes)
		return nil
	}

	if err = git.UnpackObjects(f, dir); err != nil {
		a.SetResponseRejected(ins, "cannot unpack git objects").Annotate(Annotation{"error-msg": err.Error()})
		return nil
	}

	if has_wants {
		// check out wanted digest
		notes.Add("checkout", wants[0])
		err = git.Checkout(dir, wants[0])
		if err != nil {
			return fmt.Errorf("git checkout error: %w", err)
		}
	} else {
		// check out wanted-ref
		a.SetResponseRejected(ins,
			"want-refs handling not implemented yet").Annotate(notes)
		return nil
	}

	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		a.SetResponseUnknown(ins,
			"git repository does not contain a go.mod file")
		return nil
	}

	mod := goMod{}
	if err := mod.parse(goModPath); err != nil {
		a.SetResponseRejected(ins, "cannot parse go.mod file").Annotate(
			Annotation{"error-msg": err.Error()},
		)
	}

	md := ArtefactMetadata{Type: mimetypes.GoModuleGit}

	parts := strings.Split(mod.Name, "/")
	n := len(parts)
	if n > 1 {
		md.Name = parts[n-1]
		md.Vendor = parts[n-2]
	} else {
		md.Name = mod.Name
	}

	tags, ok := a.ResponseAnnotation(GitUploadPackID, "tags")
	if ok {
		md.Version = getVersionTag(tags.(map[string]string), wants[0])
	}

	license, _ := utils.CheckLicenseFiles(
		[]string{
			filepath.Join(dir, "LICENSE"),
			filepath.Join(dir, "COPYING"),
			filepath.Join(dir, "License"),
			filepath.Join(dir, "Copying"),
		},
	)
	md.License = license

	notes.Add("module", mod.Name)
	if mod.GoVersion != "" {
		notes.Add("go", mod.GoVersion)
	}

	a.SetArtefactMetadata(md)

	// Reject if depth > 1
	if !isShallow {
		a.SetResponseRejected(ins, "go module found but repository is not shallow").Annotate(notes)
		return nil
	}

	// Reject if multiple wants
	if len(wants)+len(want_refs) > 1 {
		a.SetResponseRejected(ins, "go module found with multiple refs").Annotate(notes)
		return nil
	}

	// Reject if version not found
	if md.Version == "" {
		a.SetResponseRejected(ins, "cannot find go module version tag").Annotate(notes)
		return nil
	}

	a.SetResponseApproved(ins, "go module found").Annotate(notes)

	return nil
}

type goMod struct {
	Name      string
	GoVersion string
}

func (m *goMod) parse(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	sc := bufio.NewScanner(f)
	sc.Split(bufio.ScanLines)

	for sc.Scan() {
		line := sc.Text()

		if strings.HasPrefix(line, "module ") {
			m.Name = strings.Trim(line[7:], `"`)
			continue
		}

		if strings.HasPrefix(line, "go ") {
			m.GoVersion = line[3:]
			continue
		}

		if strings.HasPrefix(line, "require ") {
			break
		}
	}

	return nil
}

func getVersionTag(haystack map[string]string, needle string) string {
	for k, v := range haystack {
		if v == needle {
			return k
		}
	}

	return ""
}
