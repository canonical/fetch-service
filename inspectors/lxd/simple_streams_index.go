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
	"net/url"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
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
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	info, err := NewSimpleStreamsIndexUrlInfo(u)
	if err != nil {
		return nil // we don't recognize this request
	}

	a.SetRequestPending(ins, "valid Simple Streams index URL").Annotate(
		Annotation{
			"stream": info.Stream,
		},
	)
	return nil // we don't recognize this request
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
	slog.Debugf("parsing Simple Streams Index for stream %s", stream)

	decoder := json.NewDecoder(f)
	var b simpleStreamsIndex
	if err := decoder.Decode(&b); err != nil {
		return nil // we don't recognize this artifact
	}

	// Verify this is a format the inspector understands
	if b.Format != indexFormat {
		slog.Debugf("unsupported format when parsing index.json %s", b.Format)
		return nil
	}
	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.SimpleStreams,
		Name:        "Simple Streams Index",
		Description: fmt.Sprintf("Simple Streams Index for %s", stream),
	})

	var downloadPaths = make([]string, 0, len(b.Index))
	for _, v := range b.Index {
		if v.Format != productFormat {
			a.SetResponseRejected(ins, "invalid index file").Annotate(Annotation{"index.format": v.Format})
			return nil
		}
		if v.Datatype != dataType {
			a.SetResponseRejected(ins, "invalid index file").Annotate(Annotation{"index.datatype": v.Datatype})
			return nil
		}
		downloadPaths = append(downloadPaths, v.Path)
	}

	a.SetResponseApproved(ins, "valid Simple Streams index file").Annotate(
		Annotation{
			"download-paths": downloadPaths,
		})

	return nil
}
