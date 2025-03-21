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

package cargo

import (
	"encoding/json"
	"fmt"
	"regexp"

	. "github.com/canonical/fetch-service/inspectors/common"
)

var (
	// Location of the crate index (will be configurable)
	indexRequestOrigin = regexp.MustCompile(`^https://index.crates.io:443`)
	// Slug for the "root" config.json
	indexConfigSlug = regexp.MustCompile(`/config.json$`)
	// Slug for the index for a particular crate
	// example: /ti/me/time
	indexCrateSlug = regexp.MustCompile(`/\w\w/\w\w/([\w-]+)$`)
)

// CargoIndexInspector inspects Cargo crate indices downloaded from a registry.
type CargoIndexInspector struct {
}

func NewIndexInspector() *CargoIndexInspector {
	return &CargoIndexInspector{}
}

func (CargoIndexInspector) ID() string {
	return "cargo.index"
}

func (ins *CargoIndexInspector) InspectRequest(a RequestArtifact) error {
	url := a.DownloadURL()
	origin := indexRequestOrigin.FindString(url)
	if origin == "" {
		// bad origin
		return nil
	}

	// is it a request for config.json?
	config := indexConfigSlug.FindString(url)
	if config != "" {
		a.SetRequestPending(ins, "request matches valid URL").Annotate(
			Annotation{
				"is-config":  true,
				"crate-name": "",
			},
		)
		return nil
	}

	// is it a request for a crate index?
	m := indexCrateSlug.FindStringSubmatch(url)

	if len(m) == 2 {
		crate_name := m[1]
		// Request marked as Unknown because it comes from the default crates.io origin
		a.SetRequestUnknown(ins, "unsupported origin").Annotate(
			Annotation{
				"is-config":  false,
				"crate-name": crate_name,
			},
		)
		return nil
	}

	return nil
}

func (ins *CargoIndexInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	config, ok := a.RequestBoolAnnotation(ins.ID(), "is-config")
	if !ok {
		return nil
	}

	if config {
		md, err := handleConfigArtifact(f, a)
		if err != nil {
			return err
		}

		if md != nil {
			a.SetArtifactMetadata(*md)
			a.SetResponseApproved(ins, "document contains valid config.json")
		}

		return nil
	}

	crate, crate_ok := a.RequestStringAnnotation(ins.ID(), "crate-name")

	if !crate_ok {
		return nil
	}

	md, err := handleCrateArtifact(f, a)
	if err != nil {
		return err
	}

	if md != nil && md.Name == crate {
		a.SetArtifactMetadata(*md)
		a.SetResponseApproved(ins, "document contains valid crate index")
	}

	return nil

}

func handleConfigArtifact(f ArtifactReader, a ResponseArtifact) (*ArtifactMetadata, error) {
	if !a.MimetypeIs("application/json") {
		return nil, nil
	}
	type Config struct {
		Dl  string
		Api string
	}
	read := Config{}

	dec := json.NewDecoder(f)
	if err := dec.Decode(&read); err != nil {
		return nil, nil
	}

	md := ArtifactMetadata{
		Type:        "application/json",
		Name:        "config.json for Cargo package index",
		Description: "config.json for Cargo package index",
		Vendor:      read.Api,
		Author:      read.Api,
	}

	return &md, nil
}

/*
{"name":"time","vers":"0.0.1","deps":[{"name":"gcc","req":"*","features":[],"optional":false,"default_features":true,"target":null,"kind":"normal"}],"cksum":"a623e34ae3050ff0e09f6488bb2cc5440bd7ec2b3596286683026bbd697bb447","features":{},"yanked":true}
*/
func handleCrateArtifact(f ArtifactReader, a ResponseArtifact) (*ArtifactMetadata, error) {
	if !a.MimetypeIs("application/x-ndjson") {
		return nil, nil
	}

	type CrateEntry struct {
		Name string
		Vers string
	}
	entry := CrateEntry{}

	dec := json.NewDecoder(f)
	if err := dec.Decode(&entry); err != nil {
		return nil, nil
	}

	md := ArtifactMetadata{
		Type:        "application/x-ndjson",
		Name:        entry.Name,
		Description: fmt.Sprintf(`Cargo package index for crate "%s"`, entry.Name),
	}

	return &md, nil
}
