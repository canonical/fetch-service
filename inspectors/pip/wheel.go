// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2025 Canonical Ltd.
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
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/utils"
)

type WheelInspector struct {
}

func NewWheelInspector() *WheelInspector {
	return &WheelInspector{}
}

func (WheelInspector) ID() string {
	return "pip.wheel"
}

// InspectRequest verifies if the request complies with policy.
func (ins *WheelInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	if checkWheelURL(u) == nil {
		// Request marked as Unknown because it comes from the default pypi origin
		a.SetRequestUnknown(ins, "unsupported origin")
	}

	return nil // we don't recognize this request
}

var wheelPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\w+-[^/]+\.dist-info/WHEEL$`),
	regexp.MustCompile(`^\w+-[^/]+\.dist-info/METADATA$`),
	regexp.MustCompile(`^\w+-[^/]+\.dist-info/RECORD$`),
}

// InspectArtifact extracts metadata from a known artifact file format.
func (ins *WheelInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if !a.MimetypeIs("application/zip") {
		return nil
	}

	slog := a.Logger()

	// Check if zip file contains wheel files
	if !utils.ZipMatches(f, int64(f.Len()), wheelPatterns) {
		return nil
	}

	// Check if zip file contains wheel files
	if !utils.ZipMatches(f, int64(f.Len()), wheelPatterns) {
		return nil
	}

	size := int64(f.Len())
	notes := newWheelNotes()

	if err := readWheelMetadata(ins, f, size, a, notes, slog); err != nil {
		return err
	}

	fileList, err := listWheelFiles(ins, f, size, a, notes)
	if err != nil {
		return err
	}

	if err := readWheelRecord(ins, f, size, a, fileList, notes); err != nil {
		return err
	}

	processOpinion(ins, a, notes)

	return nil
}

func processOpinion(ins *WheelInspector, a ResponseArtifact, notes *wheelNotes) {
	// Reject if required files not found
	if len(notes.requirementFaults) > 0 {
		notes.Add("faults", notes.requirementFaults)
		a.SetResponseRejected(ins,
			"wheel file requirements not met").Annotate(notes.Annotation)
		return
	}

	// Reject if wheel integrity check fails
	if len(notes.integrityFaults) > 0 || len(notes.missingFiles) > 0 || len(notes.extraFiles) > 0 {
		if len(notes.integrityFaults) > 0 {
			notes.Add("faults", notes.integrityFaults)
		}
		if len(notes.missingFiles) > 0 {
			notes.Add("missing-files", notes.missingFiles)
		}
		if len(notes.extraFiles) > 0 {
			notes.Add("extra-files", notes.extraFiles)
		}
		a.SetResponseRejected(ins,
			"wheel file parsed but failed integrity verification").Annotate(notes.Annotation)
		return
	}

	a.SetResponseApproved(ins, "wheel file successfully parsed").Annotate(notes.Annotation)
}

// readWheelMetadata reads the wheel's METADATA file.
func readWheelMetadata(ins *WheelInspector, f io.ReaderAt, size int64, a ResponseArtifact, notes *wheelNotes, slog logger.Logger) error {
	z, err := zip.NewReader(f, size)
	if err != nil {
		return err
	}

	mre := regexp.MustCompile(`^\w+-[^/]+\.dist-info/METADATA$`)

	for _, f := range z.File {
		if m := mre.MatchString(f.Name); m {
			zf, err := f.Open()
			if err != nil {
				return err
			}
			defer func() { _ = zf.Close() }()

			md, ver, err := scanWheelMetadata(zf, slog)
			if err != nil {
				return err
			}
			if md.Name == "" {
				notes.requirementFault("wheel name not found")
			}
			if md.Version == "" {
				notes.requirementFault("wheel version not found")
			}
			if ver == "" {
				notes.requirementFault("wheel metadata version not found")
				return nil
			}

			a.SetArtifactMetadata(md)
			notes.Add("metadata-version", ver)
			return nil
		}
	}

	notes.requirementFault("METADATA file not found")

	return nil
}

// scanWheelMetadata parses metadata entries from the given file.
func scanWheelMetadata(zf io.ReadCloser, slog logger.Logger) (ArtifactMetadata, string, error) {
	sc := bufio.NewScanner(zf)
	sc.Split(bufio.ScanLines)

	temp, err := os.CreateTemp("", "tmpfile-")
	if err != nil {
		return ArtifactMetadata{}, "", err
	}
	defer func() { _ = temp.Close() }()
	defer func() { _ = os.Remove(temp.Name()) }()

	// create a temporary copy of the manifest for license verification
	t := bufio.NewWriter(temp)

	var mver, license string
	var name, version, description, vendor, author, email, maintainer string

	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			break
		}

		if _, err := fmt.Fprintln(t, line); err != nil {
			return ArtifactMetadata{}, "", err
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
			vendor = v
		case "author-email":
			email = v
		case "maintainer":
			maintainer = v
		}
	}

	_ = t.Flush()
	_ = temp.Close()

	license, err = utils.GetLicense(temp.Name(), slog)
	if err != nil {
		return ArtifactMetadata{}, mver, err
	}

	// If vendor is not specified, fall back to maintainer
	if vendor == "" && maintainer != "" {
		vendor = maintainer
	}

	md := ArtifactMetadata{
		Type:        mimetypes.PythonWheel,
		Name:        name,
		Version:     version,
		Description: description,
		Author:      author,
		AuthorEmail: email,
		Vendor:      vendor,
		License:     license,
	}

	return md, mver, nil
}

// memberFile is used to check integrity of the payload files.
type memberFile struct {
	Name   string               `json:"name"`   // The file name with path
	Sha256 digests.Sha256Digest `json:"sha256"` // The SHA256 digest of content
	Size   int64                `json:"size"`   // The file size
}

// listWheelFiles gets a list of wheel files and their sha1 digests.
func listWheelFiles(ins *WheelInspector, f io.ReaderAt, size int64, a ResponseArtifact, notes *wheelNotes) ([]memberFile, error) {
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
		defer func() { _ = zf.Close() }()

		if f.FileInfo().IsDir() {
			continue
		}

		sum := sha256.New()

		if _, err := io.Copy(sum, zf); err != nil {
			return res, err
		}

		res = append(res, memberFile{
			Name:   f.Name,
			Sha256: *(*digests.Sha256Digest)(sum.Sum(nil)),
			Size:   f.FileInfo().Size(),
		})
	}

	notes.Add("files", len(res))

	return res, nil
}

// readWheelRecord reads the wheel's RECORD file and verifies the
// checksum of the listed files.
func readWheelRecord(ins *WheelInspector, f io.ReaderAt, size int64, a ResponseArtifact, files []memberFile, notes *wheelNotes) error {
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
			defer func() { _ = zf.Close() }()

			if err := checkRecord(ins, zf, f.Name, a, files, notes); err != nil {
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
func checkRecord(ins *WheelInspector, zf io.ReadCloser, rname string, a ResponseArtifact, files []memberFile, notes *wheelNotes) error {
	sc := bufio.NewScanner(zf)
	sc.Split(bufio.ScanLines)

	fileMap := map[string]memberFile{}
	for _, f := range files {
		fileMap[f.Name] = f
	}

	num := 0
	for sc.Scan() {
		line := sc.Text()
		x := strings.Split(line, ",")
		if len(x) != 3 {
			notes.integrityFault("malformed RECORD entry: '%s'", line)
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
			notes.integrityFault("%s: malformed size '%s' in RECORD", name, x[2])
			return nil
		}

		// check file size
		f := fileMap[name]
		if int64(size) != f.Size {
			notes.integrityFault("%s: file size %d does not match recorded size %d", name, f.Size, size)
			return nil
		}

		// check file digest
		d := strings.SplitN(digest, "=", 2)
		if d[0] != "sha256" {
			notes.integrityFault("%s: unknown digest type '%s'", name, d[0])
			return nil
		}
		dst, err := base64.RawURLEncoding.DecodeString(d[1])
		if err != nil {
			notes.integrityFault("%s: digest decode error: %v", name, err)
			return nil
		}

		member, ok := fileMap[name]
		if !ok {
			notes.missingFile(name)
			return nil
		}
		recordDigest := digests.Sha256Digest{}
		copy(recordDigest[:], dst)
		if member.Sha256 != recordDigest {
			notes.integrityFault("%s: digest mismatch", name)
			return nil
		}

		delete(fileMap, name)
		num++
	}

	notes.Add("parsed-record-entries", num)

	for k := range fileMap {
		notes.extraFile(k)
	}

	return nil
}

// Annotation helper for wheel inspection

type wheelNotes struct {
	Annotation                 // inspection annotations
	requirementFaults []string // missing requirements to be a valid wheel file
	integrityFaults   []string // integrity errors in wheel file
	missingFiles      []string // files missing from the wheel file
	extraFiles        []string // extra files found in the wheel file
}

func newWheelNotes() *wheelNotes {
	return &wheelNotes{
		Annotation:        Annotation{},
		requirementFaults: []string{},
		integrityFaults:   []string{},
		missingFiles:      []string{},
		extraFiles:        []string{},
	}
}

func (t *wheelNotes) requirementFault(msg string, args ...any) {
	t.requirementFaults = append(t.requirementFaults, fmt.Sprintf(msg, args...))
}

func (t *wheelNotes) integrityFault(msg string, args ...any) {
	t.integrityFaults = append(t.integrityFaults, fmt.Sprintf(msg, args...))
}

func (t *wheelNotes) missingFile(name string) {
	t.missingFiles = append(t.missingFiles, name)
}

func (t *wheelNotes) extraFile(name string) {
	t.extraFiles = append(t.extraFiles, name)
}
