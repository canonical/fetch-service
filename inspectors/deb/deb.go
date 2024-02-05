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

package deb

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/zstd"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/metadata"
)

type DebInspector struct {
}

func NewDebInspector() *DebInspector {
	return &DebInspector{}
}

func (DebInspector) ID() string {
	return "deb"
}

func (ins DebInspector) InspectRequest(a *metadata.Artefact) error {
	validReqs := []*regexp.Regexp{
		regexp.MustCompile(`http://archive\.ubuntu\.com/`),
		regexp.MustCompile(`http://security\.ubuntu\.com/`),
		regexp.MustCompile(`https://esm\.ubuntu\.com:443/`),
		regexp.MustCompile(`http://repo.ros2.org/`),
	}

	for _, re := range validReqs {
		if re.MatchString(a.CurrentDownload.URL) {
			a.Consider(ins, "URL matches expression '%s'", re)
			return nil
		}
	}
	return nil // we don't recognize this request
}

func (ins *DebInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
	if a.Metadata.Type != mimetypes.DebianBinaryPackage {
		return nil
	}

	if err := ins.readDebMetadata(f, a); err != nil {
		return err
	}

	if a.Metadata.Name != "" && a.Metadata.Version != "" && a.Metadata.Architecture != "" {
		a.Approve(ins, "deb package parsed")
	}

	return nil
}

// readDebMetadata reads metadata from the deb control file.
func (ins DebInspector) readDebMetadata(f io.Reader, a *metadata.Artefact) error {
	af := ar.NewReader(f)

	for {
		h, err := af.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		switch h.Name {
		case "debian-binary":
			if ver := ins.getDebianBinaryVersion(af, a); ver != "2.0" {
				a.Reject(ins, "unknown debian binary version %q", ver)
			}
		case "control.tar.gz":
			zf, err := gzip.NewReader(af)
			if err != nil {
				return err
			}
			err = ins.parseControlTar(zf, a)
			if err != nil {
				return err
			}
		case "control.tar.zst", "control.tar.zstd":
			zf, err := zstd.NewReader(af, zstd.WithDecoderConcurrency(1))
			if err != nil {
				return err
			}
			err = ins.parseControlTar(zf, a)
			if err != nil {
				return err
			}
		case "data.tar.zst", "data.tar.zstd":
			zf, err := zstd.NewReader(af, zstd.WithDecoderConcurrency(1))
			if err != nil {
				return err
			}
			err = ins.parseDataTar(zf, a)
			if err != nil {
				return err
			}
			// TODO: add gzip reader
		}
	}

	return nil
}

func (ins DebInspector) getDebianBinaryVersion(af io.Reader, a *metadata.Artefact) string {
	sc := bufio.NewScanner(af)
	sc.Split(bufio.ScanLines)

	// Read a single line
	sc.Scan()
	return strings.TrimSpace(sc.Text())
}

func (ins DebInspector) parseControlTar(zf io.Reader, a *metadata.Artefact) error {
	tf := tar.NewReader(zf)
	for {
		h, err := tf.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		switch h.Name {
		case "./control":
			err = ins.parseControl(tf, a)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (ins DebInspector) parseControl(tf io.Reader, a *metadata.Artefact) error {
	sc := bufio.NewScanner(tf)
	sc.Split(bufio.ScanLines)

	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			break
		}

		// Skip long description
		if line[0] == ' ' {
			continue
		}

		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)

		switch k {
		case "Package":
			a.Metadata.Name = v
		case "Version":
			a.Metadata.Version = v
		case "Architecture":
			a.Metadata.Architecture = v
		case "Description":
			runes := []rune(v)
			runes[0] = unicode.ToUpper(runes[0])
			a.Metadata.Description = string(runes)
		case "Maintainer":
			a.Metadata.Vendor = v
			a.Metadata.AuthorEmail = v
		}
	}

	if a.Metadata.Name == "" || a.Metadata.Version == "" {
		a.Reject(ins, "package name/version not in control file")
	}

	return nil
}

func (ins DebInspector) parseDataTar(zf io.Reader, a *metadata.Artefact) error {
	copyright := regexp.MustCompile(`^\./usr/share/doc/[^/]+/copyright$`)

	tf := tar.NewReader(zf)
	for {
		h, err := tf.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		switch {
		case copyright.MatchString(h.Name):
			err = ins.parseCopyright(tf, a)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (ins DebInspector) parseCopyright(tf io.Reader, a *metadata.Artefact) error {
	sc := bufio.NewScanner(tf)
	sc.Split(bufio.ScanLines)

	for sc.Scan() {
		line := sc.Text()

		// Skip long description
		if len(line) > 0 && line[0] == ' ' {
			continue
		}

		// TODO: parse copyright

		k, v, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}

		switch k {
		case "License":
			a.Metadata.License = v
		case "Upstream-Contact", "Upstream author":
			a.Metadata.Author = v
		}
	}

	return nil
}
