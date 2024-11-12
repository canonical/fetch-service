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

package deb

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/zstd"

	"github.com/canonical/fetch-service/inspectors/apt/config"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/utils"
)

type DebInspector struct {
	config config.AptInspectorConfig
}

func NewDebInspector(cfg config.AptInspectorConfig) *DebInspector {
	return &DebInspector{config: cfg}
}

func (DebInspector) ID() string {
	return "deb"
}

func (ins *DebInspector) InspectRequest(a RequestArtefact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if info, err := config.NewDebPackageUrlInfo(u, &ins.config); err == nil {
		a.SetRequestPending(ins, "valid URL for deb package").Annotate(
			Annotation{
				"repository":   info.Repository,
				"component":    info.Component,
				"name":         info.Name,
				"version":      info.Version,
				"architecture": info.Architecture,
			},
		)
	}

	return nil
}

func (ins *DebInspector) InspectArtefact(f ArtefactReader, a ResponseArtefact) error {
	if !a.MimetypeIs(mimetypes.DebianBinaryPackage) {
		return nil
	}

	var md ArtefactMetadata

	if err := ins.readDebMetadata(f, &md); err != nil {
		a.SetArtefactMetadata(md)
		a.SetResponseRejected(ins, err.Error())
	} else {
		a.SetArtefactMetadata(md)
		a.SetResponseApproved(ins, "deb package successfully parsed")
	}

	return nil
}

// readDebMetadata reads metadata from the deb control file.
func (ins *DebInspector) readDebMetadata(f io.Reader, md *ArtefactMetadata) error {
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
			if ver := ins.getDebianBinaryVersion(af); ver != "2.0" {
				return fmt.Errorf("unknown debian binary version '%s'", ver)
			}
		case "control.tar.gz":
			zf, err := gzip.NewReader(af)
			if err != nil {
				return err
			}
			if err = ins.parseControlTar(zf, md); err != nil {
				return err
			}
		case "control.tar.zst", "control.tar.zstd":
			zf, err := zstd.NewReader(af, zstd.WithDecoderConcurrency(1))
			if err != nil {
				return err
			}
			if err = ins.parseControlTar(zf, md); err != nil {
				return err
			}
		case "data.tar.zst", "data.tar.zstd":
			zf, err := zstd.NewReader(af, zstd.WithDecoderConcurrency(1))
			if err != nil {
				return err
			}
			if err = ins.parseDataTar(zf, md); err != nil {
				return err
			}
			// TODO: add gzip reader
		}
	}

	return nil
}

func (ins DebInspector) getDebianBinaryVersion(af io.Reader) string {
	sc := bufio.NewScanner(af)
	sc.Split(bufio.ScanLines)

	// Read a single line
	sc.Scan()
	return strings.TrimSpace(sc.Text())
}

func (ins DebInspector) parseControlTar(zf io.Reader, md *ArtefactMetadata) error {
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
			err = ins.parseControl(tf, md)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (ins DebInspector) parseControl(tf io.Reader, md *ArtefactMetadata) error {
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

	if md.Name == "" || md.Version == "" {
		return errors.New("package name and version not listed in control file")
	}

	return nil
}

func (ins DebInspector) parseDataTar(zf io.Reader, md *ArtefactMetadata) error {
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
			err = ins.parseCopyright(tf, md)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (ins DebInspector) parseCopyright(tf io.Reader, md *ArtefactMetadata) error {
	sc := bufio.NewScanner(tf)
	sc.Split(bufio.ScanLines)

	temp, err := os.CreateTemp("", "tmpfile-")
	if err != nil {
		return err
	}
	defer temp.Close()
	defer os.Remove(temp.Name())

	// create a temporary copy for license verification
	t := bufio.NewWriter(temp)

	for sc.Scan() {
		line := sc.Text()

		if _, err := fmt.Fprintln(t, line); err != nil {
			return err
		}

		// Skip multilines lines
		if len(line) > 0 && line[0] == ' ' {
			continue
		}

		k, v, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}

		switch k {
		case "Upstream-Contact", "Upstream author":
			md.Author = v
		}
	}

	t.Flush()
	temp.Close()

	md.License, err = utils.GetLicense(temp.Name())
	if err != nil {
		return err
	}

	return nil
}
