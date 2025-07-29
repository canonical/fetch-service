// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2025 Canonical Ltd.
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

package lxd

import (
	"encoding/json"
	"fmt"
	"regexp"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
)

var (
	indexRequestURL = regexp.MustCompile(`^https://cloud-images.ubuntu.com:443/([\w-\/]+)/streams/v1/index.json$`)
)

const (
	indexFormat   = "index:1.0"
	productFormat = "products:1.0"
	dataType      = "image-downloads"
)

type SimpleStreamsIndexInspector struct {
}

func NewSimpleStreamsIndexInspector() *SimpleStreamsIndexInspector {
	return &SimpleStreamsIndexInspector{}
}

func (SimpleStreamsIndexInspector) ID() string {
	return "lxd.simple-streams.index"
}

func (ins *SimpleStreamsIndexInspector) InspectRequest(a RequestArtifact) error {
	m := indexRequestURL.FindStringSubmatch(a.DownloadURL())
	if len(m) > 1 {
		// Annotate the stream as it comes from cloud images
		a.SetRequestPending(ins, "cloud image").Annotate(
			Annotation{
				"match":  indexRequestURL,
				"stream": m[1],
			},
		)
	}

	return nil
}

type simpleStreamsIndex struct {
	Updated string                         `json:"updated"`
	Format  string                         `json:"format"`
	Index   map[string]simpleStreamEntries `json:"index"`
}

type simpleStreamEntries struct {
	Datatype string   `json:"datatype"`
	Path     string   `json:"path"`
	Updated  string   `json:"updated"`
	Products []string `json:"products"`
	Format   string   `json:"format"`
}

func (ins *SimpleStreamsIndexInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/json") {
		return nil
	}

	slog := a.Logger()

	stream, ok := a.RequestStringAnnotation(ins.ID(), "stream")
	if !ok {
		return nil
	}
	slog.Debugf("parsing index for stream %s", stream)

	decoder := json.NewDecoder(f)
	var b simpleStreamsIndex
	if err := decoder.Decode(&b); err != nil {
		return nil // we don't recognize this artifact
	}

	// Verify this is a format the inspector understands
	if b.Format != indexFormat {
		a.SetResponseRejected(ins, fmt.Sprintf("invalid index format %s", b.Format))
		return nil
	}

	var downloadPaths = make([]string, 0, len(b.Index))
	// Check that the Datatype is what we expect
	// com.ubuntu.cloud:daily:download
	for _, v := range b.Index {
		if v.Format != productFormat {
			a.SetResponseRejected(ins, fmt.Sprintf("invalid product format %s", v.Format))
			return nil
		}
		if v.Datatype != dataType {
			a.SetResponseRejected(ins, fmt.Sprintf("invalid datatype %s", v.Datatype))
			return nil
		}
		downloadPaths = append(downloadPaths, v.Path)
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.SimpleStreams,
		Name:        "Simplestreams Index",
		Description: fmt.Sprintf("SimpleStreams Index for %s", stream),
	})
	a.SetResponseApproved(ins, "valid Simple Streams index file").Annotate(
		Annotation{
			"download-paths": downloadPaths,
		})

	return nil
}
