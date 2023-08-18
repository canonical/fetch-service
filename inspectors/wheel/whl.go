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
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/utils"
)

func WhlDetector(raw []byte, limit uint32) bool {
	return utils.ZipMatches(raw,
		`[^/]*\.dist-info/WHEEL`,
		`[^/]*\.dist-info/METADATA`,
		`[^/]*\.dist-info/RECORD`)
}

type WhlInspector struct{}

func (WhlInspector) AuthorizeRequest(req *http.Request) error {
	return nil
}

func (WhlInspector) Inspect(filename string, md *metadata.Metadata, di *metadata.DownloadInfo) (stop bool, err error) {
	if md.Type != "application/x-python-wheel" {
		return
	}

	err = readWhlMetadata(filename, md)
	if err != nil {
		return
	}

	err = readWhlWheel(filename, md)
	if err != nil {
		return
	}

	fileList, err := listWheelFiles(filename)
	if err != nil {
		return
	}
	md.Files = fileList

	err = readWhlRecord(filename, md)
	if err != nil {
		return
	}

	stop = true
	return
}

// listWheelFiles gets a list of wheel files and their sha1 digests.
func listWheelFiles(filename string) ([]metadata.MemberFile, error) {
	res := []metadata.MemberFile{}

	z, err := zip.OpenReader(filename)
	if err != nil {
		return res, err
	}
	defer z.Close()

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

		res = append(res, metadata.MemberFile{
			Name:   f.Name,
			Sha256: *(*metadata.Sha256Digest)(sum.Sum(nil)),
			Size:   f.FileInfo().Size(),
		})
	}

	return res, nil
}

// readWhlMetadata reads the wheel's METADATA file.
func readWhlMetadata(filename string, md *metadata.Metadata) error {
	z, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer z.Close()

	mre := regexp.MustCompile(`^[^/]*\.dist-info/METADATA$`)

	for _, f := range z.File {
		if m := mre.MatchString(f.Name); m {
			zf, err := f.Open()
			if err != nil {
				return err
			}
			defer zf.Close()

			ver := scanManifest(zf, md)
			md.Annotate("wheel.metadata", metadata.AnnotationValue{"version": ver})
			break
		}
	}

	return nil
}

// scanManifest parses metadata entries from the given file.
func scanManifest(zf io.ReadCloser, md *metadata.Metadata) string {
	sc := bufio.NewScanner(zf)
	sc.Split(bufio.ScanLines)

	var ver string

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
			normalizeClassifier(v, md)
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
func normalizeClassifier(v string, md *metadata.Metadata) {
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
func readWhlWheel(filename string, md *metadata.Metadata) error {
	z, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer z.Close()

	record := regexp.MustCompile(`^[^/]*\.dist-info/WHEEL$`)

	for _, f := range z.File {
		if m := record.MatchString(f.Name); m {
			zf, err := f.Open()
			if err != nil {
				return err
			}
			defer zf.Close()

			sc := bufio.NewScanner(zf)
			sc.Split(bufio.ScanLines)

			entries := metadata.AnnotationValue{}

			for sc.Scan() {
				line := sc.Text()
				k, v, ok := strings.Cut(line, ": ")
				if !ok {
					continue
				}
				entries[k] = v
			}

			md.Annotate("wheel.details", entries)
			break
		}
	}

	return nil
}

// readWhlRecord reads the wheel's RECORD file.
func readWhlRecord(filename string, md *metadata.Metadata) error {
	z, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer z.Close()

	record := regexp.MustCompile(`^[^/]*\.dist-info/RECORD$`)

	for _, f := range z.File {
		if m := record.MatchString(f.Name); m {
			zf, err := f.Open()
			if err != nil {
				return err
			}
			defer zf.Close()

			if err := checkRecord(zf, f.Name, md); err != nil {
				return err
			}
			break
		}
	}

	return nil
}

// checkRecord verifies files against the RECORD file checksum.
func checkRecord(zf io.ReadCloser, rname string, md *metadata.Metadata) error {
	sc := bufio.NewScanner(zf)
	sc.Split(bufio.ScanLines)

	fileMap := map[string]metadata.MemberFile{}
	for _, f := range md.Files {
		fileMap[f.Name] = f
	}

	messages := []string{}

	for sc.Scan() {
		line := sc.Text()
		x := strings.Split(line, ",")
		if len(x) != 3 {
			messages = append(messages, fmt.Sprintf("malformed RECORD entry: '%s'", line))
			continue
		}
		name, digest := x[0], x[1]
		if digest == "" && name == rname {
			continue
		}
		size, err := strconv.Atoi(x[2])
		if err != nil {
			messages = append(messages, fmt.Sprintf("%s: malformed size '%s' in RECORD", name, x[2]))
			continue
		}

		// check file size
		f := fileMap[name]
		if int64(size) != f.Size {
			messages = append(messages, fmt.Sprintf(
				"%s: file size %d does not match recorded size %d", name, f.Size, size))
		}

		// check file digest
		d := strings.SplitN(digest, "=", 2)
		if d[0] != "sha256" {
			messages = append(messages, fmt.Sprintf("%s: unknown digest type '%s'", name, d[0]))
		}
		dst, err := base64.RawURLEncoding.DecodeString(d[1])
		if err != nil {
			messages = append(messages, fmt.Sprintf("%s: digest decode error: %v", name, err))
		}

		// TODO: check extra files
		member, ok := fileMap[name]
		if !ok {
			messages = append(messages, fmt.Sprintf("%s: file not in RECORD", name))
		}
		recordDigest := metadata.Sha256Digest{}
		copy(recordDigest[:], dst)
		if member.Sha256 != recordDigest {
			messages = append(messages, fmt.Sprintf("%s: digest mismatch", name))
		}
	}

	if len(messages) > 0 {
		data := metadata.AnnotationValue{"errors": messages}
		md.Annotate("wheel.record.error", data)
	}

	return nil
}
