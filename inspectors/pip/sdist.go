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
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/opinions"
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
func (ins SdistInspector) InspectRequest(a *metadata.Artefact) error {
	u, err := url.Parse(a.CurrentDownload.URL)
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if checkSdistUrl(u) == nil {
		a.SetRequestOpinion(ins.ID(), opinions.Pending, "request matches valid URL")
	}

	return nil // we don't recognize this request
}

// InspectArtefact extracts metadata from a known artefact file format.
func (ins *SdistInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
	if !a.MimeType.Is("application/gzip") {
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
func (ins *SdistInspector) parsePkgInfo(tf io.Reader, a *metadata.Artefact) error {
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

	var mver string
	var name, version, description, author, email, maintainer string

	md := &a.Metadata

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

	md.Type = mimetypes.PythonSdist
	md.Name = name
	md.Version = version
	md.Description = description
	md.Author = author
	md.AuthorEmail = email

	md.License, err = utils.GetLicense(temp.Name())
	if err != nil {
		return err
	}

	// If vendor is not specified, fall back to maintainer
	if author != "" {
		md.Vendor = author
	} else {
		md.Vendor = maintainer
	}

	a.SetResponseOpinion(ins.ID(), opinions.Approved, "sdist file successfully parsed").Annotate(
		metadata.Annotation{
			"metadata-version": mver,
		},
	)

	return nil
}
