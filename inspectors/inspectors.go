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
	"net/http"
	"path/filepath"
	"reflect"
	//"sort"
	//"sync"

	"github.com/gabriel-vasile/mimetype"
	"github.com/go-mmap/mmap"

	//"github.com/canonical/fetch-service/inspectors/apt"
	"github.com/canonical/fetch-service/inspectors/deb"
	"github.com/canonical/fetch-service/inspectors/wheel"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
)

// Inspector is the interface implemented by artifact metadata
// extractors.
type Inspector interface {
	//
	AuthorizeRequest(*http.Request) error

	// Inspect extracts metadata from the given artifact and
	// populates the metadata structure, returning whether
	// the artifact was identified and no further examination
	// by other inspectors is required.
	Inspect(string, *metadata.Metadata, *metadata.DownloadInfo) (bool, error)
}

var (
	inspectors = []Inspector{}

	//lock             sync.Mutex
	sortedInspectors []Inspector
)

type Inspectors struct {
	insmap map[string]Inspector
	keys   []string
}

func New() Inspectors {
	insps := Inspectors{}
	insps.insmap = map[string]Inspector{}

	for _, ins := range []Inspector{
		wheel.WhlInspector{},
		deb.DebInspector{},
		//apt.AptReleaseInspector{},
		//apt.AptPackagesInspector{},
		DefaultInspector{},
	} {
		t := reflect.TypeOf(ins).String()
		insps.keys = append(insps.keys, t)
		insps.insmap[t] = ins
		logger.Debugf("register inspector %s", t)
	}

	return insps
}

// Run executes the registered inspectors for the artifact in the
// given directory, populating the metadata structure md.
func (insps Inspectors) Run(dir string, md *metadata.Metadata, di *metadata.DownloadInfo) error {
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
		stop, err := insps.insmap[key].Inspect(filename, md, di)
		if err != nil {
			return err
		}
		if stop {
			break
		}
	}

	return nil
}

/*
func ensureSortedInspectors() {
	if sortedInspectors == nil {
		sortedInspectors = sortInspectors()
	}
}

func sortInspectors() []Inspector {
	keys := make([]string, 0, len(inspectors))
	for k := range inspectors {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	values := make([]Inspector, len(keys)+1)
	for i, k := range keys {
		values[i] = inspectors[k]
	}
	values[len(keys)] = defaultInspector

	return values
}
*/

// DefaultInspector is a fallback artifact inspector for unknown file
// formats.
type DefaultInspector struct{}

func (DefaultInspector) AuthorizeRequest(req *http.Request) error {
	return nil
}

func (ins DefaultInspector) Inspect(filename string, md *metadata.Metadata, di *metadata.DownloadInfo) (bool, error) {
	md.Annotate("default.format.unknown", metadata.AnnotationValue{})
	return true, nil
}
