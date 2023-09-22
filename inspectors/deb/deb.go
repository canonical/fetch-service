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
	"github.com/canonical/fetch-service/metadata"
)

type DebInspector struct{}

func (DebInspector) ID() string {
	return "deb"
}

func (ins *DebInspector) InitializeContext(sd SessionDetails) {
}

func (DebInspector) InspectRequest(a *metadata.Artefact) error {
	return nil
}

func (DebInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) (stop bool, err error) {
	if a.Metadata.Type != "application/vnd.debian.binary-package" {
		return
	}
	stop = true

	err = readDebMetadata(f, &a.Metadata)
	if err != nil {
		return
	}

	/*
		pkgsDigest, e, ok := ctx.GetPackagesEntry(md.Sha256)
		if ok {
			if md.Name != e.Package || md.Version != e.Version || md.Architecture != e.Architecture || md.Size != e.Size {
				data := metadata.AnnotationValue{"packages-data": e}
				md.Annotate(IntegrityViolation, "file.integrity.check", ResultFail).SetDetails(data)
				return
			}
			md.Annotate(Notice, "file.integrity.asserted-by", pkgsDigest.String())
		} else {
			md.Annotate(PolicyViolation, "file.integrity.check", ResultFail)
		}
	*/

	return
}

func (ins DebInspector) API() InspectorAPI {
	return nil
}

// readDebMetadata reads metadata from the deb control file.
func readDebMetadata(f io.Reader, md *metadata.Metadata) error {

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
			err = parseDebianBinary(af, md)
			if err != nil {
				return err
			}
		case "control.tar.gz":
			zf, err := gzip.NewReader(af)
			if err != nil {
				return err
			}
			err = parseControlTar(zf, md)
			if err != nil {
				return err
			}
		case "control.tar.zst", "control.tar.zstd":
			zf, err := zstd.NewReader(af, zstd.WithDecoderConcurrency(1))
			if err != nil {
				return err
			}
			err = parseControlTar(zf, md)
			if err != nil {
				return err
			}
		case "data.tar.zst", "data.tar.zstd":
			zf, err := zstd.NewReader(af, zstd.WithDecoderConcurrency(1))
			if err != nil {
				return err
			}
			err = parseDataTar(zf, md)
			if err != nil {
				return err
			}
			// TODO: add gzip reader
		}
	}

	return nil
}

func parseDebianBinary(af io.Reader, md *metadata.Metadata) error {
	sc := bufio.NewScanner(af)
	sc.Split(bufio.ScanLines)

	// Read a single line
	sc.Scan()
	line := sc.Text()
	value := metadata.AnnotationValue{"version": line}
	md.Annotate("deb.debian-binary.details", value)

	return nil
}

func parseControlTar(zf io.Reader, md *metadata.Metadata) error {
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
			err = parseControl(tf, md)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func parseControl(tf io.Reader, md *metadata.Metadata) error {
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
			md.Name = v
		case "Version":
			md.Version = v
		case "Architecture":
			md.Architecture = v
		case "Description":
			runes := []rune(v)
			runes[0] = unicode.ToUpper(runes[0])
			md.Description = string(runes)
		case "Maintainer":
			md.Vendor = v
			md.AuthorEmail = v
		}
	}

	return nil
}

func parseDataTar(zf io.Reader, md *metadata.Metadata) error {
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
			err = parseCopyright(tf, md)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func parseCopyright(tf io.Reader, md *metadata.Metadata) error {
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
			md.License = v
		case "Upstream-Contact", "Upstream author":
			md.Author = v
		}
	}

	return nil
}
