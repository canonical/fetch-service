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

	"github.com/canonical/fetch-service/inspectors/api"
	"github.com/canonical/fetch-service/inspectors/apt"
	"github.com/canonical/fetch-service/inspectors/deb"
	"github.com/canonical/fetch-service/inspectors/wheel"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
)

// Inspector is the interface implemented by artifact metadata
// extractors.
type Inspector interface {
	ID() string

	InitializeContext(sd api.SessionDetails)

	InspectRequest(*metadata.Artefact) error

	// Inspect extracts metadata from the given artifact and
	// populates the metadata structure, returning whether
	// the artifact was identified and no further examination
	// by other inspectors is required.
	InspectArtefact(*mmap.File, *metadata.Metadata, *metadata.DownloadInfo) (bool, error)

	API() api.InspectorAPI
}

type Inspectors struct {
	insmap map[string]Inspector
	keys   []string
}

func New(sd api.SessionDetails) Inspectors {
	insps := Inspectors{}
	insps.insmap = map[string]Inspector{}

	for _, ins := range []Inspector{
		&wheel.WhlInspector{},
		&deb.DebInspector{},
		&apt.AptReleaseInspector{},
		&apt.AptPackagesInspector{},
		&DefaultInspector{},
	} {
		ins.InitializeContext(sd)
		id := ins.ID()
		insps.keys = append(insps.keys, id)
		insps.insmap[id] = ins
		logger.Debugf("register inspector: %s", id)
	}

	return insps
}

func (insps Inspectors) RunRequestInspectors(a *metadata.Artefact) error {
	for _, key := range insps.keys {
		ins := insps.insmap[key]
		logger.Debugf("run request inspector: %s", ins.ID())
		err := ins.InspectRequest(a)
		if err != nil {
			return err
		}
	}

	return nil
}

// RunArtefactInspectors examines the artefact in the given assets directory.
func (insps Inspectors) RunArtefactInspectors(dir string, a *metadata.Artefact) error {
	md := a.ArtefactMetadata()
	di := a.RequestMetadata()

	// detect file type
	filename := filepath.Join(dir, fmt.Sprintf("%s.data", md.Sha256))
	f, err := mmap.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	mtype, err := mimetype.DetectReader(f)
	if err != nil {
		return err
	}

	md.Type = mtype.String()

	if len(di.ContentType) > 0 && !mtype.Is(di.ContentType) {
		logger.Debugf("file type '%s' doesn't match content type '%s'", mtype.String(), di.ContentType)
	}

	// run metadata inspectors
	for _, key := range insps.keys {
		ins := insps.insmap[key]
		logger.Debugf("run artefact inspector: %s", ins.ID())
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		stop, err := ins.InspectArtefact(f, md, di)
		if err != nil {
			return err
		}
		if stop {
			break
		}
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

// DefaultInspector is a fallback artifact inspector for unknown file
// formats.
type DefaultInspector struct{}

func (DefaultInspector) ID() string {
	return "default"
}

func (DefaultInspector) InitializeContext(sd api.SessionDetails) {
}

func (DefaultInspector) InspectRequest(a *metadata.Artefact) error {
	return nil
}

func (DefaultInspector) InspectArtefact(f *mmap.File, md *metadata.Metadata, di *metadata.DownloadInfo) (bool, error) {
	md.Annotate("default.format.unknown", metadata.AnnotationValue{})
	return true, nil
}

func (DefaultInspector) API() api.InspectorAPI {
	return nil
}
