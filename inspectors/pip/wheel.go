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

package pip

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/utils"
)

var (
	// FIXME: using PyPI URLs as placeholders
	reRequestURL = regexp.MustCompile(
		`^https://files\.pythonhosted\.org:443/packages/[0-9a-f]{2}/[0-9a-f]{2}/[0-9a-f]{60}/\w+-[a-zA-Z0-9\.-]+\.whl`)
)

func WheelDetector(raw []byte, limit uint32) bool {
	return utils.ZipMatches(raw,
		`^\w+-[^/]+\.dist-info/WHEEL$`,
		`^\w+-[^/]+\.dist-info/METADATA$`,
		`^\w+-[^/]+\.dist-info/RECORD$`)
}

type WheelInspector struct {
}

func NewWheelInspector() *WheelInspector {
	return &WheelInspector{}
}

func (WheelInspector) ID() string {
	return "pip.wheel"
}

// InspectRequest verifies if the request complies with policy.
func (ins WheelInspector) InspectRequest(a *metadata.Artefact) error {
	if reRequestURL.MatchString(a.CurrentDownload.URL) {
		a.AuthorizeRequest(ins)
	}

	return nil // we don't recognize this request
}

// InspectArtefact extracts metadata from a known artefact file format.
func (ins *WheelInspector) InspectArtefact(f ReadAtSeeker, a *metadata.Artefact) error {
	md := a.Metadata
	if md.Type != mimetypes.PythonWheel {
		return nil
	}

	size := int64(f.Len())

	ver, err := readWheelMetadata(ins, f, size, a)
	if err != nil {
		return err
	}

	fileList, err := listWheelFiles(ins, f, size, a)
	if err != nil {
		return err
	}

	if err := readWheelRecord(ins, f, size, a, fileList); err != nil {
		return err
	}

	if a.Rejected() {
		return nil
	}

	a.Approve(ins, "wheel file successfully parsed, metadata version %s", ver)
	return nil
}

// readWheelMetadata reads the wheel's METADATA file.
func readWheelMetadata(ins *WheelInspector, f io.ReaderAt, size int64, a *metadata.Artefact) (string, error) {
	z, err := zip.NewReader(f, size)
	if err != nil {
		return "", err
	}

	mre := regexp.MustCompile(`^\w+-[^/]+\.dist-info/METADATA$`)

	for _, f := range z.File {
		if m := mre.MatchString(f.Name); m {
			zf, err := f.Open()
			if err != nil {
				return "", err
			}
			defer zf.Close()

			ver, err := scanWheelMetadata(zf, a)
			if err != nil {
				return "", err
			}
			if a.Metadata.Name == "" {
				a.Reject(ins, "wheel name not found")
			}
			if a.Metadata.Version == "" {
				a.Reject(ins, "wheel version not found")
			}
			if ver == "" {
				a.Reject(ins, "wheel metadata version not found")
				return "", nil
			}
			return ver, nil
		}
	}

	a.Reject(ins, "METADATA file not found")
	return "", nil
}

// scanWheelMetadata parses metadata entries from the given file.
func scanWheelMetadata(zf io.ReadCloser, a *metadata.Artefact) (string, error) {
	sc := bufio.NewScanner(zf)
	sc.Split(bufio.ScanLines)

	temp, err := os.CreateTemp("", "tmpfile-")
	if err != nil {
		return "", err
	}
	defer temp.Close()
	defer os.Remove(temp.Name())

	// create a temporary copy of the manifest for license verification
	t := bufio.NewWriter(temp)

	var ver string
	var maintainer string

	md := &a.Metadata

	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			break
		}

		if _, err := fmt.Fprintln(t, line); err != nil {
			return "", err
		}

		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)

		switch strings.ToLower(k) {
		case "metadata-version":
			ver = v
		case "name":
			md.Name = v
		case "version":
			md.Version = v
		case "summary":
			md.Description = v
		case "author":
			md.Author = v
			md.Vendor = v
		case "author-email":
			md.AuthorEmail = v
		case "maintainer":
			maintainer = v
		}
	}

	t.Flush()
	temp.Close()

	md.License, err = utils.GetLicense(temp.Name())
	if err != nil {
		return ver, err
	}

	// If vendor is not specified, fall back to maintainer
	if md.Vendor == "" && maintainer != "" {
		md.Vendor = maintainer
	}

	return ver, nil
}

// memberFile is used to check integrity of the payload files.
type memberFile struct {
	Name   string                `json:"name"`   // The file name with path
	Sha256 metadata.Sha256Digest `json:"sha256"` // The SHA256 digest of content
	Size   int64                 `json:"size"`   // The file size
}

// listWheelFiles gets a list of wheel files and their sha1 digests.
func listWheelFiles(ins *WheelInspector, f io.ReaderAt, size int64, a *metadata.Artefact) ([]memberFile, error) {
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

// readWheelRecord reads the wheel's RECORD file and verifies the
// checksum of the listed files.
func readWheelRecord(ins *WheelInspector, f io.ReaderAt, size int64, a *metadata.Artefact, files []memberFile) error {
	z, err := zip.NewReader(f, size)
	if err != nil {
		return err
	}

	record := regexp.MustCompile(`^\w+-[^/]*\.dist-info/RECORD$`)

	found := false
	for _, f := range z.File {
		if m := record.MatchString(f.Name); m {
			found = true
			zf, err := f.Open()
			if err != nil {
				return err
			}
			defer zf.Close()

			if err := checkRecord(ins, zf, f.Name, a, files); err != nil {
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
func checkRecord(ins *WheelInspector, zf io.ReadCloser, rname string, a *metadata.Artefact, files []memberFile) error {
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
			a.Reject(ins, "malformed RECORD entry: '%s'", line)
			return nil
		}
		name, digest := x[0], x[1]
		if digest == "" && name == rname {
			// exclude RECORD entry
			delete(fileMap, name)
			continue
		}
		size, err := strconv.Atoi(x[2])
		if err != nil {
			a.Reject(ins, "%s: malformed size '%s' in RECORD", name, x[2])
			return nil
		}

		// check file size
		f := fileMap[name]
		if int64(size) != f.Size {
			a.Reject(ins,
				"%s: file size %d does not match recorded size %d", name, f.Size, size)
			return nil
		}

		// check file digest
		d := strings.SplitN(digest, "=", 2)
		if d[0] != "sha256" {
			a.Reject(ins, "%s: unknown digest type '%s'", name, d[0])
			return nil
		}
		dst, err := base64.RawURLEncoding.DecodeString(d[1])
		if err != nil {
			a.Reject(ins, "%s: digest decode error: %v", name, err)
			return nil
		}

		member, ok := fileMap[name]
		if !ok {
			a.Reject(ins, "%s: file missing from package ", name)
			return nil
		}
		recordDigest := metadata.Sha256Digest{}
		copy(recordDigest[:], dst)
		if member.Sha256 != recordDigest {
			a.Reject(ins, "%s: digest mismatch", name)
			return nil
		}

		delete(fileMap, name)
	}

	for k := range fileMap {
		a.Reject(ins, fmt.Sprintf("%s: file not in RECORD", k))
	}

	return nil
}
