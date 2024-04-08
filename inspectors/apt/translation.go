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

package apt

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/xi2/xz"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
)

// Check if the given data could be a valid Translation file.
// The Translation file should contain at least the following fields:
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
	if err != nil {
		return false
	}

	buf = buf[:n]

	sc := bufio.NewScanner(bytes.NewReader(buf))
	sc.Split(bufio.ScanLines)

	fields := map[string]struct{}{}

	for sc.Scan() {
		line := sc.Text()
		if len(line) == 0 {
			continue
		}

		k, _, ok := strings.Cut(line, ":")
		if !ok {
			return false
		}

		if k == "Description-md5" {
			break // We have enough data to work on
		}

		fields[k] = struct{}{}
	}

	expected_fields := []string{
		"Package",
		"Description-md5",
	}

	for _, k := range expected_fields {
		_, ok := fields[k]
		if !ok {
			logger.Debugf("expected field %q not found", k)
			return false // we don't recognize this file
		}
	}

	return true
}

// AptTranslationInspector contains inspector-specific contextual data for stateful
// analysis within a fetch session.
type AptTranslationInspector struct {
}

func NewAptTranslationInspector() *AptTranslationInspector {
	return &AptTranslationInspector{}
}

func (ins *AptTranslationInspector) ID() string {
	return "apt.translations"
}

func (ins *AptTranslationInspector) InspectRequest(a *metadata.Artefact) error {
	u, err := url.Parse(a.CurrentDownload.URL)
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if info, err := newTranslationUrlInfo(u); err == nil {
		a.Hold(ins, "valid URL for Translation file").Annotate(
			metadata.Annotation{
				"repository":   info.repository,
				"distribution": info.distribution,
				"component":    info.component,
			},
		)
	}

	return nil
}

func (ins *AptTranslationInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
	if a.Metadata.Type != mimetypes.AptTranslation {
		return nil
	}
	fSize := f.Len()

	if fSize == 0 {
		return nil
	}

	u, err := url.Parse(a.CurrentDownload.URL)
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	r, err := xz.NewReader(f, 0)
	if err != nil {
		return err
	}

	sc := bufio.NewScanner(r)
	sc.Split(bufio.ScanLines)

	// some lines can be really long
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	item_count := 0
	state_package := false
	state_md5sum := false
	state_description := false
	lang := ""

	// Check if the Translation file is well-formed
	for sc.Scan() {
		line := sc.Text()

		if strings.HasPrefix(line, "Package: ") {
			if state_package {
				a.Reject(ins, "Misplaced Package fields in Translation file")
				return nil
			}
			state_package = true
			continue
		} else if strings.HasPrefix(line, "Description-md5: ") {
			if !state_package {
				a.Reject(ins, "Description-md5 field without Package field")
				return nil
			}
			state_md5sum = true
			continue
		} else if strings.HasPrefix(line, "Description-") {
			if !state_md5sum || !state_package {
				a.Reject(ins, "Description field without Package or Description-md5 field")
				return nil
			}
			state_description = true
			if lang == "" { // get the language code
				descLang, _, langFound := strings.Cut(line, ":")
				if langFound {
					lang = strings.TrimPrefix(descLang, "Description-")
				}
			}
			continue
		} else if strings.HasPrefix(line, " ") { // Description field continuation with leading space
			if !state_description {
				a.Reject(ins, "Description field without Package or Description-md5 field")
				return nil
			}
			continue
		} else if len(line) == 0 { // item ends
			if state_description {
				item_count++
			}
			state_package = false
			state_md5sum = false
			state_description = false
		}
	}

	// Handle the last item if not followed by an empty line
	if item_count > 0 {
		if state_package {
			if !state_md5sum {
				a.Reject(ins, "Description-md5 field missing for the last Package")
				return nil
			}
			if !state_description {
				a.Reject(ins, "Description field missing for the last Package")
				return nil
			}
		}
	} else if fSize > 0 {
		a.Reject(ins, "Not a valid Translation file")
		return nil
	}

	a.Metadata.Name = path.Base(u.Path)

	// the file should be also annotated by the release inspector
	rins, ok := a.ResponseInspection["apt.release"]
	if ok {
		v, ok := rins.Annotations["vendor"]
		if ok {
			a.Metadata.Author = fmt.Sprintf("%s", v)
			a.Metadata.Vendor = fmt.Sprintf("%s", v)
		}
	}

	a.Approve(ins, "translations file succesfully parsed").Annotate(
		metadata.Annotation{
			"translation-language": lang,
			"translation-count":    item_count,
		},
	)

	return nil
}
