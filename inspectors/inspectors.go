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

package inspectors

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/gabriel-vasile/mimetype"

	"github.com/canonical/fetch-service/inspectors/apt"
	"github.com/canonical/fetch-service/inspectors/cargo"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/craft"
	"github.com/canonical/fetch-service/inspectors/deb"
	"github.com/canonical/fetch-service/inspectors/files"
	"github.com/canonical/fetch-service/inspectors/git"
	"github.com/canonical/fetch-service/inspectors/gomod"
	"github.com/canonical/fetch-service/inspectors/maven"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/inspectors/pip"
	"github.com/canonical/fetch-service/inspectors/snap"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/canonical/fetch-service/service/config"
)

func init() {
	mimetype.SetLimit(1 << 16) // set buffer size to 64Kb
	mimetype.Lookup("application/x-xz").Extend(apt.AptPackagesDetector, mimetypes.AptPackages, "")
	mimetype.Lookup("application/x-xz").Extend(apt.AptTranslationDetector, mimetypes.AptTranslation, "")
	mimetype.Lookup("application/octet-stream").Extend(snap.SquashFsDetector, mimetypes.SquashFs, "")
	mimetype.Lookup("text/plain").Extend(snap.AssertionDetector, mimetypes.Assertion, ".assert")
}

type Inspectors struct {
	insmap     map[string]Inspector
	ids        []string
	permissive bool
}

func New(permissive bool, cfg config.InspectorsConfig) Inspectors {

	insList := []Inspector{
		// snap
		snap.NewSnapInspector(cfg.Snap),
		snap.NewSnapAssertionInspector(),
		snap.NewSnapInfoInspector(),
		snap.NewSnapRefreshInspector(),

		// python
		pip.NewSimpleIndexInspector(),
		pip.NewWheelInspector(),
		pip.NewSdistInspector(),
		pip.NewMetadataInspector(),

		// deb packages
		deb.NewDebInspector(cfg.Apt),
		apt.NewAptReleaseInspector(cfg.Apt),
		apt.NewAptPackagesInspector(cfg.Apt),
		apt.NewAptTranslationInspector(cfg.Apt),

		// git
		git.NewSmartQueryInspector(cfg.Git),
		git.NewUploadPackInspector(cfg.Git),

		// craft
		// must run after git
		craft.NewSourcecraftInspector(cfg.Crafts),
		craft.NewRockcraftInspector(cfg.Crafts),
		craft.NewSnapcraftInspector(cfg.Crafts),

		// go
		// must run after git
		gomod.NewGoModuleGitInspector(),

		// rust
		cargo.NewIndexInspector(),
		cargo.NewCrateInspector(),

		// maven
		maven.NewJarInspector(),
		maven.NewPomInspector(),

		// default inspector
		// must be the last inspector to run
		DefaultInspector{},
	}

	insNum := len(insList)

	insps := Inspectors{
		insmap:     make(map[string]Inspector, insNum),
		ids:        make([]string, insNum),
		permissive: permissive,
	}

	for n, ins := range insList {
		id := ins.ID()
		insps.ids[n] = id
		insps.insmap[id] = ins
		//logger.Debugf("register inspector: %s", id)
	}

	return insps
}

// RunRequestInspectors determine whether the HTTP request is valid.
func (insps Inspectors) RunRequestInspectors(a *metadata.Artifact) error {
	logger.Debugf("Inspect request: %s", a.CurrentDownload.URL)
	for _, id := range insps.ids {
		ins := insps.insmap[id]
		logger.Debugf("run request inspector: %s", ins.ID())
		if err := ins.InspectRequest(a); err != nil {
			a.SetRequestRejected(ins, "error inspecting request").Annotate(
				Annotation{"error-message": err.Error()})
			return err
		}
	}

	return nil
}

// RunArtifactInspectors examines the artifact in the given assets directory.
func (insps Inspectors) RunArtifactInspectors(dir string, a *metadata.Artifact) error {
	// detect file type
	filename := filepath.Join(dir, fmt.Sprintf("%s.data", a.Metadata.Sha256))
	logger.Debugf("run artifact inspectors on %s", filename)

	f, err := files.OpenArtifactFile(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	mtype, err := mimetype.DetectReader(f)
	if err != nil {
		logger.Debug("cannot detect mime type")
		return err
	}

	a.Metadata.Type = mtype.String()
	a.MimeType = mtype
	ctype := a.CurrentDownload.ContentType

	if len(ctype) > 0 && !mtype.Is(ctype) {
		logger.Debugf("file type '%s' doesn't match content type '%s'", mtype.String(), ctype)
	}

	// run artifact inspectors
	for _, id := range insps.ids {
		// if not permissive, only inspectors with pending opinions can run
		// (the default inspector always runs)
		if !insps.permissive && id != "default" {
			reqin, ok := a.RequestInspection[id]
			if !ok || reqin.Opinion != opinions.Pending {
				continue
			}
		}

		ins := insps.insmap[id]
		logger.Debugf("run artifact inspector: %s", id)
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if err := ins.InspectArtifact(f, a); err != nil {
			a.SetResponseRejected(ins, "error inspecting artifact").Annotate(
				Annotation{"error-message": err.Error()})
			return err
		}
	}

	return nil
}

func (insps Inspectors) GetInspector(id string) (Inspector, error) {
	ins, ok := insps.insmap[id]
	if !ok {
		return nil, fmt.Errorf("inspector '%s' not registered", id)
	}

	return ins, nil
}

// List returns the list of all registered inspector IDs.
func (insps Inspectors) List() []string {
	return insps.ids
}

// DefaultInspector is a fallback inspector for unknown requests or artifacts.
type DefaultInspector struct {
}

func (ins DefaultInspector) ID() string {
	return "default"
}

func (ins DefaultInspector) InspectRequest(a RequestArtifact) error {
	if !a.RequestRejected() && !a.RequestPending() {
		a.SetRequestUnknown(ins, "the request was not recognized by any format inspector")
	}
	return nil
}

func (ins DefaultInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.ResponseRejected() && !a.ResponseApproved() {
		a.SetResponseUnknown(ins, "the artifact file content was not recognized by any format inspector")
	}
	return nil
}
