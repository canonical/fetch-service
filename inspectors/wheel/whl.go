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

package wheel

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/utils"
)

func WhlDetector(raw []byte, limit uint32) bool {
	return utils.ZipMatches(raw,
		`[^/]*\.dist-info/WHEEL`,
		`[^/]*\.dist-info/METADATA`,
		`[^/]*\.dist-info/RECORD`)
}

type WhlInspector struct {
}

func (WhlInspector) ID() string {
	return "wheel"
}

func (ins WhlInspector) InitializeContext(sd SessionDetails) {
}

func (ins WhlInspector) InspectRequest(a *metadata.Artefact) error {
	return nil
}

func (ins *WhlInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) (stop bool, err error) {
	md := a.Metadata

	if md.Type != mimetypes.PythonWheel {
		return
	}

	size := int64(f.Len())

	err = ins.readWhlMetadata(f, size, a)
	if err != nil {
		return
	}

	err = ins.readWhlWheel(f, size, a)
	if err != nil {
		return
	}

	fileList, err := ins.listWheelFiles(f, size, a)
	if err != nil {
		return
	}

	err = ins.readWhlRecord(f, size, a, fileList)
	if err != nil {
		return
	}

	stop = true
	return
}

func (ins WhlInspector) API() InspectorAPI {
	return nil
}

type memberFile struct {
	Name   string                `json:"name"`   // The file name with path
	Sha256 metadata.Sha256Digest `json:"sha256"` // The SHA256 digest of content
	Size   int64                 `json:"size"`   // The file size
}

// listWheelFiles gets a list of wheel files and their sha1 digests.
func (ins WhlInspector) listWheelFiles(f io.ReaderAt, size int64, a *metadata.Artefact) ([]memberFile, error) {
	res := []memberFile{}

	z, err := zip.NewReader(f, size)
	if err != nil {
		return res, err
	}

	for _, f := range z.File {
		zf, err := f.Open()
		if err != nil {
			return res, err
		}
		defer zf.Close()

		if f.FileInfo().IsDir() {
			continue
		}

		sum := sha256.New()

		if _, err := io.Copy(sum, zf); err != nil {
			return res, err
		}

		res = append(res, memberFile{
			Name:   f.Name,
			Sha256: *(*metadata.Sha256Digest)(sum.Sum(nil)),
			Size:   f.FileInfo().Size(),
		})
	}

	return res, nil
}

// readWhlMetadata reads the wheel's METADATA file.
func (ins WhlInspector) readWhlMetadata(f io.ReaderAt, size int64, a *metadata.Artefact) error {
	z, err := zip.NewReader(f, size)
	if err != nil {
		return err
	}

	mre := regexp.MustCompile(`^[^/]*\.dist-info/METADATA$`)

	for _, f := range z.File {
		if m := mre.MatchString(f.Name); m {
			zf, err := f.Open()
			if err != nil {
				return err
			}
			defer zf.Close()

			ver := scanManifest(zf, a)
			if ver != "" {
				a.Approve(ins, "found wheel metadata version %s", ver)
			} else {
				a.Reject(ins, "wheel metadata version not found")
			}
			break
		}
	}

	return nil
}

// scanManifest parses metadata entries from the given file.
func scanManifest(zf io.ReadCloser, a *metadata.Artefact) string {
	sc := bufio.NewScanner(zf)
	sc.Split(bufio.ScanLines)

	var ver string
	md := &a.Metadata

	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			break
		}

		k, v, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}

		switch strings.ToLower(k) {
		case "metadata-version":
			ver = v
		case "name":
			md.Name = v
		case "version":
			md.Version = v
		case "summary":
			md.Description = v
		case "license-expression":
			md.License = v
		case "classifier":
			normalizeClassifier(v, a)
		case "author":
			md.Author = v
			md.Vendor = v
		case "author-email": // FIXME: normalize author name and email
			md.AuthorEmail = v
		}
	}

	return ver
}

// normalizeClassifier converts Classifier manifest entries.
func normalizeClassifier(v string, a *metadata.Artefact) {
	md := a.Metadata
	parts := strings.Split(v, " :: ")
	if len(parts) < 2 {
		return
	}
	switch parts[0] {
	case "License":
		md.License = parts[len(parts)-1]
	}
}

// readWhlWheel reads the wheel's WHEEL file.
func (ins WhlInspector) readWhlWheel(f io.ReaderAt, size int64, a *metadata.Artefact) error {
	z, err := zip.NewReader(f, size)
	if err != nil {
		return err
	}

	record := regexp.MustCompile(`^[^/]*\.dist-info/WHEEL$`)

	found := false
	for _, f := range z.File {
		if m := record.MatchString(f.Name); m {
			found = true
			break
		}
	}

	if !found {
		a.Reject(ins, "WHEEL file not found")
		return fmt.Errorf("WHEEL file not found")
	}

	a.Approve(ins, "WHEEL file found")

	return nil

	//			zf, err := f.Open()
	//			if err != nil {
	//				return err
	//			}
	//			defer zf.Close()
	//
	//			sc := bufio.NewScanner(zf)
	//			sc.Split(bufio.ScanLines)
	//
	//			entries := metadata.AnnotationValue{}
	//
	//			for sc.Scan() {
	//				line := sc.Text()
	//				k, v, ok := strings.Cut(line, ": ")
	//				if !ok {
	//					continue
	//				}
	//				entries[k] = v
	//			}
	//
	//			md.Annotate("wheel.details", entries)
	//			break
}

// readWhlRecord reads the wheel's RECORD file.
func (ins WhlInspector) readWhlRecord(f io.ReaderAt, size int64, a *metadata.Artefact, files []memberFile) error {
	z, err := zip.NewReader(f, size)
	if err != nil {
		return err
	}

	record := regexp.MustCompile(`^[^/]*\.dist-info/RECORD$`)

	found := false
	for _, f := range z.File {
		if m := record.MatchString(f.Name); m {
			found = true
			zf, err := f.Open()
			if err != nil {
				return err
			}
			defer zf.Close()

			if err := checkRecord(zf, f.Name, a, files); err != nil {
				return err
			}
			break
		}
	}

	if !found {
		return fmt.Errorf("RECORD file not found")
	}

	return nil
}

// checkRecord verifies files against the RECORD file checksum.
func checkRecord(zf io.ReadCloser, rname string, a *metadata.Artefact, files []memberFile) error {
	sc := bufio.NewScanner(zf)
	sc.Split(bufio.ScanLines)

	fileMap := map[string]memberFile{}
	for _, f := range files {
		fileMap[f.Name] = f
	}

	for sc.Scan() {
		line := sc.Text()
		x := strings.Split(line, ",")
		if len(x) != 3 {
			return fmt.Errorf("malformed RECORD entry: '%s'", line)
		}
		name, digest := x[0], x[1]
		if digest == "" && name == rname {
			continue
		}
		size, err := strconv.Atoi(x[2])
		if err != nil {
			return fmt.Errorf("%s: malformed size '%s' in RECORD", name, x[2])
		}

		// check file size
		f := fileMap[name]
		if int64(size) != f.Size {
			return fmt.Errorf(
				"%s: file size %d does not match recorded size %d", name, f.Size, size)
		}

		// check file digest
		d := strings.SplitN(digest, "=", 2)
		if d[0] != "sha256" {
			return fmt.Errorf("%s: unknown digest type '%s'", name, d[0])
		}
		dst, err := base64.RawURLEncoding.DecodeString(d[1])
		if err != nil {
			return fmt.Errorf("%s: digest decode error: %v", name, err)
		}

		// TODO: check extra files
		member, ok := fileMap[name]
		if !ok {
			return fmt.Errorf("%s: file not in RECORD", name)
		}
		recordDigest := metadata.Sha256Digest{}
		copy(recordDigest[:], dst)
		if member.Sha256 != recordDigest {
			return fmt.Errorf("%s: digest mismatch", name)
		}
	}

	return nil
}
