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
	"errors"
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

// AptCommandsDetector checks if the given raw data could be a valid commands file.
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

	fields, ok := getTextFields(buf, 3)
	if !ok {
		return false
	}

	expectedFields := []string{
		"suite",
		"component",
		"arch",
	}

	for _, k := range expectedFields {
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

	sl := a.Logger()

	if info, err := apt_cfg.NewCommandURLInfo(u, &ins.config, sl); err == nil {
		a.SetRequestPending(ins, "valid URL for Commands file").Annotate(
			Annotation{
				"repository": info.Repository,
				"suite":      info.Suite,
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

	md := ArtifactMetadata{
		Type:        mimetypes.AptCommands,
		Name:        "Commands",
		Description: "Commands list for command-not-found",
	}

	r, err := xz.NewReader(f, 0)
	if err != nil {
		a.SetResponseRejected(ins, "cannot read xz file", md)
		return nil
	}

	sc := bufio.NewScanner(r)

	// some lines can be really long
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	suite, component, arch, err := parseHeader(sc)
	if err != nil {
		a.SetResponseRejected(ins, err.Error(), md)
		return nil
	}

	md.AptSuite = suite

	sc.Scan() // Skip empty line

	itemCount, err := parsePkgList(sc)
	if err != nil {
		a.SetResponseRejected(ins, err.Error(), md)
		return nil
	}

	// the file should be also annotated by the release inspector
	vendor, ok := a.ResponseStringAnnotation(aptReleaseInspectorID, "vendor")
	if ok {
		md.Vendor = vendor
		md.Author = vendor
	}

	notes := Annotation{
		"suite":     suite,
		"component": component,
		"arch":      arch,
		"count":     itemCount,
	}

	_, ok = a.ResponseStringAnnotation(aptReleaseInspectorID, "release-file")
	if ok {
		a.SetResponseApproved(ins, "commands file successfully parsed", md).Annotate(notes)
	} else {
		a.SetResponseRejected(ins, "commands file not verified against release file", md).Annotate(notes)
	}

	return nil
}

func parseHeader(sc *bufio.Scanner) (string, string, string, error) {
	suite := ""
	component := ""
	arch := ""
	errIllFormed := errors.New("ill-formed commands file header")
	for sc.Scan() {
		line := sc.Text()
		if len(line) == 0 {
			break
		}

		k, v, ok := strings.Cut(line, ":")
		if !ok {
			return "", "", "", errIllFormed
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
		return "", "", "", errIllFormed
	}

	return suite, component, arch, nil
}

func parsePkgList(sc *bufio.Scanner) (int, error) {
	itemCount := 0
	stateName := false
	stateVersion := false
	stateCommands := false
	for sc.Scan() {
		line := sc.Text()

		if strings.HasPrefix(line, "name: ") {
			if stateName {
				return itemCount, errors.New("duplicate name field in commands file")
			}
			stateName = true
			continue
		} else if strings.HasPrefix(line, "version: ") {
			if stateVersion {
				return itemCount, errors.New("duplicate version field in commands file")
			}
			stateVersion = true
			continue
		} else if strings.HasPrefix(line, "commands: ") {
			if stateCommands {
				return itemCount, errors.New("duplicate commands field in commands file")
			}
			stateCommands = true
			continue
		} else if len(line) == 0 { // item ends
			if !stateName || !stateVersion || !stateCommands {
				return itemCount, errors.New("ill-formed entry in commands file")
			}
			itemCount++
			stateName = false
			stateVersion = false
			stateCommands = false
		}
	}

	return itemCount, nil
}
