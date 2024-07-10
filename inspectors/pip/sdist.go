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

package pip

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/utils"
)

type SdistInspector struct {
}

func NewSdistInspector() *SdistInspector {
	return &SdistInspector{}
}

func (SdistInspector) ID() string {
	return "pip.sdist"
}

// InspectRequest verifies if the request complies with policy.
func (ins *SdistInspector) InspectRequest(a RequestArtefact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if checkSdistUrl(u) == nil {
		a.SetRequestPending(ins, "request matches valid URL")
	}

	return nil // we don't recognize this request
}

// InspectArtefact extracts metadata from a known artefact file format.
func (ins *SdistInspector) InspectArtefact(f ArtefactReader, a ResponseArtefact) error {
	if !a.MimetypeIs("application/gzip") {
		return nil
	}

	zf, err := gzip.NewReader(f)
	if err != nil {
		return err
	}

	rePkgInfo := regexp.MustCompile(`^\w+-[0-9A-Za-z\.-]+/PKG-INFO$`)

	// Parse tarball
	tf := tar.NewReader(zf)
	for {
		h, err := tf.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			logger.Debugf("sdist tar parsing error: %s", err)
			return nil // we don't recognize this artefact
		}
		if rePkgInfo.MatchString(h.Name) {
			return ins.parsePkgInfo(tf, a)
		}
	}

	return nil
}

// scanSdistMetadata parses metadata entries from the given file.
func (ins *SdistInspector) parsePkgInfo(tf io.Reader, a ResponseArtefact) error {
	sc := bufio.NewScanner(tf)
	sc.Split(bufio.ScanLines)

	temp, err := os.CreateTemp("", "tmpfile-")
	if err != nil {
		return err
	}
	defer temp.Close()
	defer os.Remove(temp.Name())

	// create a temporary copy of the PKG-INFO for license verification
	t := bufio.NewWriter(temp)

	var mver, license string
	var name, version, description, author, email, vendor, maintainer string

	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			break
		}

		if _, err := fmt.Fprintln(t, line); err != nil {
			return err
		}

		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)

		switch strings.ToLower(k) {
		case "metadata-version":
			mver = v
		case "name":
			name = v
		case "version":
			version = v
		case "summary":
			description = v
		case "author":
			author = v
		case "author-email":
			email = v
		case "maintainer":
			maintainer = v
		}
	}

	t.Flush()
	temp.Close()

	if mver == "" || name == "" || version == "" {
		return nil
	}

	license, err = utils.GetLicense(temp.Name())
	if err != nil {
		return err
	}

	// If vendor is not specified, fall back to maintainer
	if author != "" {
		vendor = author
	} else {
		vendor = maintainer
	}

	a.SetArtefactMetadata(ArtefactMetadata{
		Type:        mimetypes.PythonSdist,
		Name:        name,
		Version:     version,
		Description: description,
		Author:      author,
		AuthorEmail: email,
		License:     license,
		Vendor:      vendor,
	})

	a.SetResponseApproved(ins, "sdist file successfully parsed").Annotate(
		Annotation{
			"metadata-version": mver,
		},
	)

	return nil
}
