// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2026 Canonical Ltd.
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

// AptTranslationDetector checks if the given data could be a valid translation file.
// The translation file should contain the following fields:
// - Package
// - Description-md5
// - Description-<lang>
func AptTranslationDetector(raw []byte, limit uint32) bool {
	r, err := xz.NewReader(bytes.NewReader(raw), 0)
	if err != nil {
		return false
	}

	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}

	buf = buf[:n]

	fields, ok := getTextFields(buf, 3)
	if !ok {
		return false
	}

	expectedFields := []string{
		"Package",
		"Description-md5",
	}

	for _, k := range expectedFields {
		_, ok := fields[k]
		if !ok {
			logger.Debugf("apt translation detector: expected field %q not found", k)
			return false // we don't recognize this file
		}
	}

	return true
}

// getTextFields looks for n key: value lines in buf.
func getTextFields(buf []byte, n int) (map[string]struct{}, bool) {
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
			return fields, false
		}

		fields[k] = struct{}{}
	}

	if len(fields) != n {
		return fields, false
	}

	return fields, true
}

// AptTranslationInspector contains inspector-specific contextual data for stateful
// analysis within a fetch session.
type AptTranslationInspector struct {
	config apt_cfg.AptInspectorConfig
}

func NewAptTranslationInspector(cfg apt_cfg.AptInspectorConfig) *AptTranslationInspector {
	return &AptTranslationInspector{config: cfg}
}

func (ins *AptTranslationInspector) ID() string {
	return "apt.translations"
}

func (ins *AptTranslationInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	sl := a.Logger()

	if info, err := apt_cfg.NewTranslationURLInfo(u, &ins.config, sl); err == nil {
		a.SetRequestPending(ins, "valid URL for Translation file").Annotate(
			Annotation{
				"repository": info.Repository,
				"suite":      info.Suite,
				"component":  info.Component,
			},
		)
	}

	return nil
}

func (ins *AptTranslationInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs(mimetypes.AptTranslation) {
		return nil
	}

	_, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	r, err := xz.NewReader(f, 0)
	if err != nil {
		a.SetResponseUnknown(ins, "cannot read xz file")
		return nil
	}

	sc := bufio.NewScanner(r)
	sc.Split(bufio.ScanLines)

	// some lines can be really long
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	lang, itemCount, err := parseTranslationFile(sc)
	if err != nil {
		a.SetResponseRejected(ins, err.Error())
		return nil
	}
	suite, ok := a.RequestStringAnnotation(ins.ID(), "suite")
	if !ok {
		a.SetResponseUnknown(ins, "suite not specified in request URL")
		return nil
	}

	md := ArtifactMetadata{
		Type:     mimetypes.AptTranslation,
		Name:     "Translation",
		AptSuite: suite,
	}

	// The file should be also annotated by the release inspector.
	vendor, ok := a.ResponseStringAnnotation(aptReleaseInspectorID, "vendor")
	if ok {
		md.Vendor = vendor
		md.Author = vendor
	}

	a.SetArtifactMetadata(md)

	notes := Annotation{
		"translation-language": lang,
		"translation-count":    itemCount,
	}

	_, ok = a.ResponseStringAnnotation(aptReleaseInspectorID, "release-file")
	if ok {
		a.SetResponseApproved(ins, "translation file successfully parsed").Annotate(notes)
	} else {
		a.SetResponseRejected(ins, "translation file not verified against release file").Annotate(notes)
	}

	return nil
}

func parseTranslationFile(sc *bufio.Scanner) (string, int, error) {
	itemCount := 0
	statePackage := false
	stateMD5sum := false
	stateDescription := false
	lang := ""

	// Check if the Translation file is well-formed
	for sc.Scan() {
		line := sc.Text()

		if strings.HasPrefix(line, "Package: ") {
			if statePackage {
				return "", itemCount, errors.New("misplaced package fields in translation file")
			}
			statePackage = true
			continue
		} else if strings.HasPrefix(line, "Description-md5: ") {
			if !statePackage {
				return "", itemCount, errors.New("description-md5 field without Package field")
			}
			stateMD5sum = true
			continue
		} else if strings.HasPrefix(line, "Description-") {
			if !stateMD5sum || !statePackage {
				return "", itemCount, errors.New("description field without Package or Description-md5 field")
			}
			stateDescription = true
			if lang == "" { // get the language code
				descLang, _, langFound := strings.Cut(line, ":")
				if langFound {
					lang = strings.TrimPrefix(descLang, "Description-")
				}
			}
			continue
		} else if strings.HasPrefix(line, " ") { // Description field continuation with leading space
			if !stateDescription {
				return "", itemCount, errors.New("description field without Package or Description-md5 field")
			}
			continue
		} else if len(line) == 0 { // item ends
			if stateDescription {
				itemCount++
			}
			statePackage = false
			stateMD5sum = false
			stateDescription = false
		}
	}

	// Handle the last item if not followed by an empty line
	if itemCount > 0 {
		if statePackage {
			if !stateMD5sum {
				return "", itemCount, errors.New("description-md5 field missing for the last Package")
			}
			if !stateDescription {
				return "", itemCount, errors.New("description field missing for the last Package")
			}
		}
	} else {
		return "", itemCount, errors.New("not a valid translation file")
	}

	return lang, itemCount, nil
}
