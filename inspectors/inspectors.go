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
	"github.com/go-mmap/mmap"

	"github.com/canonical/fetch-service/inspectors/apt"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/deb"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/inspectors/pip"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
)

func init() {
	mimetype.SetLimit(1 << 30) // input data is mmapped
	mimetype.Lookup("application/zip").Extend(pip.WheelDetector, mimetypes.PythonWheel, ".whl")
	mimetype.Lookup("text/plain").Extend(apt.AptReleaseDetector, mimetypes.AptRelease, "")
	mimetype.Lookup("application/x-xz").Extend(apt.AptPackagesDetector, mimetypes.AptPackages, "")
}

// Inspector is the interface implemented by artefact metadata
// extractors.
type Inspector interface {
	ID() string

	InspectRequest(*metadata.Artefact) error

	// Inspect extracts metadata from the given artefact and
	// populates the metadata structure, returning whether
	// the artefact was identified and no further examination
	// by other inspectors is required.
	InspectArtefact(ReadAtSeeker, *metadata.Artefact) error
}

type Inspectors struct {
	insmap     map[string]Inspector
	ids        []string
	permissive bool
}

func New(permissive bool) Inspectors {

	insList := []Inspector{
		pip.NewSimpleIndexInspector(),
		pip.NewWheelInspector(),
		deb.NewDebInspector(),
		apt.NewAptReleaseInspector(),
		apt.NewAptPackagesInspector(),
		DefaultInspector{permissive},
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
		logger.Debugf("register inspector: %s", id)
	}

	return insps
}

// RunRequestInspectors determine whether the HTTP request is valid.
func (insps Inspectors) RunRequestInspectors(a *metadata.Artefact) error {
	logger.Debugf("Inspect request: %s", a.CurrentDownload.URL)
	for _, id := range insps.ids {
		ins := insps.insmap[id]
		logger.Debugf("run request inspector: %s", ins.ID())
		if err := ins.InspectRequest(a); err != nil {
			return err
		}
	}

	if len(a.AuthorizedIDs) == 0 {
		url := a.CurrentDownload.URL
		return fmt.Errorf("request to %q rejected", url)
	}

	return nil
}

// RunArtefactInspectors examines the artefact in the given assets directory.
func (insps Inspectors) RunArtefactInspectors(dir string, a *metadata.Artefact) error {
	// detect file type
	filename := filepath.Join(dir, fmt.Sprintf("%s.data", a.Metadata.Sha256))
	logger.Debugf("run artefact inspectors on %s", filename)

	f, err := mmap.Open(filename)
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

	// run artefact inspectors
	// only inspectors with approved requests can inspect artefacts
	for _, id := range a.AuthorizedIDs {
		ins := insps.insmap[id]
		logger.Debugf("run artefact inspector: %s", ins.ID())
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if err := ins.InspectArtefact(f, a); err != nil {
			a.Reject(ins, err.Error())
			return err
		}
	}

	if !a.Approved() && !insps.permissive {
		return ErrRejectedArtefact
	}

	return nil
}

func (insps Inspectors) GetInspector(name string) (Inspector, error) {
	ins, ok := insps.insmap[name]
	if !ok {
		return nil, fmt.Errorf("inspector '%s' not registered", name)
	}

	return ins, nil
}

// DefaultInspector is a fallback artefact inspector for unknown file
// formats.
type DefaultInspector struct {
	permissive bool
}

func (ins DefaultInspector) ID() string {
	return "default"
}

func (ins DefaultInspector) InspectRequest(a *metadata.Artefact) error {
	if ins.permissive {
		logger.Infof("request to %s would be rejected (permissive)", a.CurrentDownload.URL)
		a.AuthorizeRequest(ins)
	}
	return nil
}

func (ins DefaultInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
	if a.Unknown() {
		a.Reject(ins, "artefact format unknown")
	}
	return nil
}
