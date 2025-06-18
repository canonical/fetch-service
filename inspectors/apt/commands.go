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

package apt

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/xi2/xz"

	apt_cfg "github.com/canonical/fetch-service/inspectors/apt/config"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
)

// Check if the given raw data could be a valid commands file.
func AptCommandsDetector(raw []byte, limit uint32) bool {
	r, err := xz.NewReader(bytes.NewReader(raw), 0)
	if err != nil {
		return false
	}

	buf := make([]byte, 4096)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}

	buf = buf[:n]

	sc := bufio.NewScanner(bytes.NewReader(buf))
	sc.Split(bufio.ScanLines)

	fields := map[string]struct{}{}

	for sc.Scan() {
		line := sc.Text()
		if len(line) > 0 && line[0] == ' ' {
			continue
		}
		if len(line) == 0 {
			break
		}

		k, _, ok := strings.Cut(line, ":")
		if !ok {
			return false
		}

		fields[k] = struct{}{}
	}

	if len(fields) != 3 {
		return false
	}

	expected_fields := []string{
		"suite",
		"component",
		"arch",
	}

	for _, k := range expected_fields {
		_, ok := fields[k]
		if !ok {
			logger.Debugf("apt commands detector: expected field %q not found", k)
			return false // we don't recognize this file
		}
	}

	return true
}

type AptCommandsInspector struct {
	config apt_cfg.AptInspectorConfig
}

func NewAptCommandsInspector(cfg apt_cfg.AptInspectorConfig) *AptCommandsInspector {
	return &AptCommandsInspector{config: cfg}
}

func (ins *AptCommandsInspector) ID() string {
	return "apt.commands"
}

func (ins *AptCommandsInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	slog := a.Logger()

	if info, err := apt_cfg.NewCommandsUrlInfo(u, &ins.config, slog); err == nil {
		a.SetRequestPending(ins, "valid URL for Commands file").Annotate(
			Annotation{
				"repository": info.Repository,
				"dist":       info.Dist,
				"component":  info.Component,
			},
		)
	}

	return nil
}

func (ins *AptCommandsInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs(mimetypes.AptCommands) {
		return nil
	}

	_, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	r, err := xz.NewReader(f, 0)
	if err != nil {
		a.SetResponseRejected(ins, "cannot read xz file")
		return nil
	}

	sc := bufio.NewScanner(r)

	// some lines can be really long
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	item_count := 0
	state_name := false
	state_version := false
	state_commands := false
	suite := ""
	component := ""
	arch := ""

	// Parse header
	for sc.Scan() {
		line := sc.Text()
		if len(line) == 0 {
			break
		}

		k, v, ok := strings.Cut(line, ":")
		if !ok {
			a.SetResponseRejected(ins, "ill-formed commands file header")
			return nil
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)

		switch k {
		case "suite":
			suite = v
		case "component":
			component = v
		case "arch":
			arch = v
		}
	}

	if suite == "" || component == "" || arch == "" {
		a.SetResponseRejected(ins, "ill-formed commands file header")
		return nil
	}

	sc.Scan() // Skip empty line

	// Parse package list and count entries
	for sc.Scan() {
		line := sc.Text()

		if strings.HasPrefix(line, "name: ") {
			if state_name {
				a.SetResponseRejected(ins, "duplicate name field in commands file")
				return nil
			}
			state_name = true
			continue
		} else if strings.HasPrefix(line, "version: ") {
			if state_version {
				a.SetResponseRejected(ins, "duplicate version field in commands file")
				return nil
			}
			state_version = true
			continue
		} else if strings.HasPrefix(line, "commands: ") {
			if state_commands {
				a.SetResponseRejected(ins, "duplicate commands field in commands file")
				return nil
			}
			state_commands = true
			continue
		} else if len(line) == 0 { // item ends
			if !state_name || !state_version || !state_commands {
				a.SetResponseRejected(ins, "ill-formed entry in commands file")
				return nil
			}
			item_count++
			state_name = false
			state_version = false
			state_commands = false
		}
	}

	md := ArtifactMetadata{
		Type: mimetypes.AptCommands,
		Name: "Commands",
	}

	// the file should be also annotated by the release inspector
	vendor, ok := a.ResponseStringAnnotation("apt.release", "vendor")
	if ok {
		md.Vendor = vendor
		md.Author = vendor
	}

	a.SetArtifactMetadata(md)
	a.SetResponseApproved(ins, "commands file successfully parsed").Annotate(
		Annotation{
			"suite":     suite,
			"component": component,
			"arch":      arch,
			"count":     item_count,
		},
	)

	return nil
}
