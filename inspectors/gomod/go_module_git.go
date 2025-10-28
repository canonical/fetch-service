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
	"github.com/canonical/fetch-service/inspectors/mimetypes"
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
func (ins *GoModuleGitInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if !a.RequestHeaderContains("Content-Type", "application/x-git-upload-pack-request") {
		return nil // we don't recognize this request
	}

	if !a.RequestHeaderContains("Accept", "application/x-git-upload-pack-result") {
		return nil // we don't recognize this request
	}

	_, err = newGoModuleGitURLInfo(u)
	if err != nil {
		return nil // we don't recognize this request
	}

	command, ok := a.RequestStringAnnotation(GitUploadPackID, "command")
	if !ok || command != "fetch" {
		return nil // we don't recognize this request
	}

	// Request marked as Unknown because it comes from a default origin
	a.SetRequestUnknown(ins, "unsupported origin")
	return nil
}

// InspectArtifact verifies if the fetched repository data
// contains a go module.
func (ins *GoModuleGitInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {

	if a.ContentType() != "application/x-git-upload-pack-result" {
		return nil
	}

	checkoutPath, ok := a.ResponseStringAnnotation(GitUploadPackID, "git-checkout-path")
	if !ok {
		// this must have been set by the git upload-pack inspector
		a.SetResponseUnknown(ins, "no git checkout found")
		return nil
	}
	notes := Annotation{}

	a.Logger().Debugf("inspect git upload-pack artifact: checkout at %q", checkoutPath)

	goModPath := filepath.Join(checkoutPath, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return nil
	}

	mod := goMod{}
	if err := mod.parse(goModPath); err != nil {
		a.SetResponseUnknown(ins, "cannot parse go.mod file").Annotate(
			Annotation{"error-msg": err.Error()},
		)
	}

	md := ArtifactMetadata{Type: mimetypes.GoModuleGit}

	parts := strings.Split(mod.Name, "/")
	n := len(parts)
	if n > 1 {
		md.Name = parts[n-1]
		md.Vendor = parts[n-2]
	} else {
		md.Name = mod.Name
	}

	// Read wants information from the git inspector annotation
	w, has_wants := a.RequestAnnotation(GitUploadPackID, "wants")
	if !has_wants {
		// this must have been set by the git upload-pack inspector
		return errors.New("cannot read request want annotation")
	}

	var wants []string
	if has_wants {
		var ok bool
		wants, ok = w.([]string)
		if !ok || len(wants) < 1 {
			return errors.New("cannot read want annotation")
		}
	}

	tags, ok := a.ResponseAnnotation(GitUploadPackID, "tags")
	if ok {
		md.Version = getVersionTag(tags.(map[string]string), wants[0])
	} else {
		// Cannot approve if version not found
		a.SetResponseUnknown(ins, "cannot find go module version tag").Annotate(notes)
		return nil
	}

	license, _ := utils.CheckLicenseFiles(
		[]string{
			filepath.Join(checkoutPath, "LICENSE"),
			filepath.Join(checkoutPath, "COPYING"),
			filepath.Join(checkoutPath, "License"),
			filepath.Join(checkoutPath, "Copying"),
		},
		a.Logger(),
	)
	md.License = license

	notes.Add("module", mod.Name)
	if mod.GoVersion != "" {
		notes.Add("go", mod.GoVersion)
	}

	a.SetResponseApproved(ins, "go module found").Annotate(notes)
	a.SetArtifactMetadata(md)

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
