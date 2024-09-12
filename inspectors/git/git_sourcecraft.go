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

package git

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
)

const (
	GitUploadPackID = "git.upload-pack"
)

// The SourcecraftInspector handles upload-pack requests. It recognizes
type SourcecraftInspector struct {
}

func NewSourcecraftInspector() *SourcecraftInspector {
	return &SourcecraftInspector{}
}

func (ins *SourcecraftInspector) ID() string {
	return "git.sourcecraft"
}

type sourcecraftYaml struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Summary string `json:"summary"`
	License string `json:"license,omitempty"`
	Base    string `json:"base"`
}

// TODO: check with the team if we want need to do that

// InspectRequest verifies whether this is a valid upload-pack request. For
// it to succeed the following conditions must be satisfied:
//
//   - The "Git-Protocol" request header must be set to "version=2".
//   - The Content-Type header must be set to "application/x-git-upload-pack-request".
//   - The Accept header must be set to "application/x-git-upload-pack-result"
//   - The request URL must match a valid upload-pack pattern.
//   - The upload-pack command must be "ls-refs" or "fetch".
//   - If command is "fetch", it must want a single shallow ref.
func (ins *SourcecraftInspector) InspectRequest(a RequestArtefact) error {
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

	_, err = newUploadPackUrlInfo(u)
	if err != nil {
		return nil // we don't recognize this request
	}

	command, ok := a.RequestStringAnnotation(GitUploadPackID, "command")
	if !ok || command != "fetch" {
		return nil // we don't recognize this request
	}

	a.SetRequestPending(ins, "valid URL for sourcecraft download")
	return nil
}

func (ins *SourcecraftInspector) InspectArtefact(f ArtefactReader, a ResponseArtefact) error {
	// FIXME: text/html is here temporarly, due to bug in launchpad git serving
	if a.ContentType() != "application/x-git-upload-pack-result" && a.ContentType() != "text/html" {
		logger.Debugf("Does not handle request of type %s", a.ContentType())
		return nil
	}
	logger.Debugf("Inspecting source artefact")

	command, ok := a.RequestStringAnnotation(GitUploadPackID, "command") // the upload-pack request command
	if !ok {
		// this must have been set by the git upload-pack inspector
		return errors.New("cannot read request command annotation")
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
	// Reject if depth > 1
	if !isShallow {
		a.SetResponseRejected(ins, "sourcecraft repository is not shallow").Annotate(notes)
		return nil
	}

	// Unpack and checkout in temporary directory
	dir, err := os.MkdirTemp("", "fetch-sourcecraft-")
	if err != nil {
		return err
	}
	logger.Debugf("unpack objects in %s", dir)

	defer os.RemoveAll(dir)

	if err = UnpackObjects(f, dir); err != nil {
		return fmt.Errorf("git unpack error: %w", err)
	}

	if has_wants {
		// check out wanted digest
		notes.Add("checkout", wants[0])
		err = Checkout(dir, wants[0])
		if err != nil {
			return fmt.Errorf("git checkout error: %w", err)
		}
	} else {
		// check out wanted-ref
		a.SetResponseRejected(ins,
			"want-refs handling not implemented yet").Annotate(notes)
		return nil
	}

	sourcecraftYamlPath := filepath.Join(dir, "sourcecraft.yaml")
	if _, err := os.Stat(sourcecraftYamlPath); err != nil {
		a.SetResponseUnknown(ins,
			"git repository does not contain a sourcecraft.yaml file")
		return nil
	}
	yamldata_filereader, err := os.Open(sourcecraftYamlPath)
	if err != nil {
		a.SetResponseRejected(ins, "cannot open sourcecraft.yaml file")
	}
	defer yamldata_filereader.Close()

	var data sourcecraftYaml
	dec := yaml.NewDecoder(yamldata_filereader)
	if err := dec.Decode(&data); err != nil {
		a.SetResponseRejected(ins, "cannot decode sourcecraft.yaml")
		return nil
	}

	a.SetArtefactMetadata(ArtefactMetadata{
		Type:        mimetypes.SourcecraftGit,
		Name:        data.Name,
		Version:     data.Version,
		Description: data.Summary,
		License:     data.License,
	})
	a.SetResponseApproved(ins, "sourcecraft repository found").Annotate(notes)

	return nil
}
