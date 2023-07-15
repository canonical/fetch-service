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

package metadata

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/go-mmap/mmap"
)

// AnnotationKind qualifies the annotation.
type AnnotationKind string

const (
	Notice             AnnotationKind = "notice"              // Informative note added by the inspector
	Warning            AnnotationKind = "warning"             // Non-fatal warning note added by the inspector
	PolicyViolation    AnnotationKind = "policy-violation"    // The artifact violates a policy rule
	IntegrityViolation AnnotationKind = "integrity-violation" // The artifact has an integrity issue
	Error              AnnotationKind = "error"               // Error during the inspector execution
)

// Standard results used in .check annotation values to
// tell whether the verification was successful.
const (
	ResultPass string = "pass" // check passed
	ResultFail string = "fail" // check failed
)

// AnnotationDetails is a map containing further details about
// an annotation result.
type AnnotationDetails map[string]interface{}

// Annotation contains a free-form text added by an artifact
// inspector.
type Annotation struct {
	Timestamp time.Time         `json:"time"`              // When the annotation was added
	Kind      AnnotationKind    `json:"kind"`              // The annotation kind
	Origin    string            `json:"origin"`            // The artifact inspector name
	Value     string            `json:"value"`             // Annotation value
	Details   AnnotationDetails `json:"details,omitempty"` // Optional annotation details
}

func (a *Annotation) SetDetails(data AnnotationDetails) {
	a.Details = data
}

type AnnotationMap map[string]*Annotation

const (
	MetadataVersionMajor = 0 // Updated when incompatible changes are made
	MetadataVersionMinor = 1 // Existing fields not changed, may contain additional fields
)

// Metadata holds information about each artifact.
type Metadata struct {
	MetadataVersion string         `json:"metadata-version"`       // Metadata version in X.Y format
	Type            string         `json:"type"`                   // The mime-type of the artifact file
	Sha1            string         `json:"sha1"`                   // The SHA1 digest of the artifact file
	Sha256          string         `json:"sha256"`                 // The SHA256 digest of the artifact file
	Size            int64          `json:"size"`                   // The size of the artifact file
	Name            string         `json:"name"`                   // The artifact designation, given by its author
	Version         string         `json:"version"`                // The artifact version, as published by the upstream
	Vendor          string         `json:"vendor"`                 // The artifact vendor
	Description     string         `json:"description"`            // A free-form description of the artifact
	Author          string         `json:"author"`                 // The artifact author name
	AuthorEmail     string         `json:"author-email,omitempty"` // The artifact author email address
	Architecture    string         `json:"architecture,omitempty"` // The architecture, if the artifact contains binary code
	License         string         `json:"license"`                // The license the artifact is published under
	Copyright       string         `json:"copyright,omitempty"`    // The copyright line, if available
	Annotations     AnnotationMap  `json:"annotations,omitempty"`  // Annotations added by artifact inspectors
	Downloads       []DownloadInfo `json:"downloads"`              // Information about artifact downloads
	Files           []MemberFile   `json:"files,omitempty"`        // Information about files contained in this artifact
	AssetDir        string         `json:"-"`                      // Location to store files and metadata
	Tempfile        string         `json:"-"`                      // Path to temporary file containing downloaded data
}

// Annotate adds a named annotation to the file metadata.
func (md *Metadata) Annotate(kind AnnotationKind, name, value string) *Annotation {
	origin := findCallerInspector()

	a := &Annotation{time.Now().UTC(), kind, origin, value, AnnotationDetails{}}
	if md.Annotations == nil {
		md.Annotations = AnnotationMap{}
	}
	md.Annotations[name] = a

	return a
}

// MemberFile contains information about files contained in the artifact.
type MemberFile struct {
	Name   string `json:"name"`   // The qualified file name
	Sha256 string `json:"sha256"` // The SHA256 digest of content
	Size   int64  `json:"size"`   // The file size
}

// DownloadInfo holds information about each artifact download.
type DownloadInfo struct {
	StartTime      time.Time           `json:"start-time"`      // When the downloaded started (UTC)
	EndTime        time.Time           `json:"end-time"`        // When the download finished (UTC)
	Method         string              `json:"method"`          // The HTTP request method
	URL            string              `json:"url"`             // The requested URL
	Address        string              `json:"address"`         // The HTTP client's IP address
	UserAgent      string              `json:"user-agent"`      // The HTTP client's user agent
	StatusCode     int                 `json:"status-code"`     // The HTTP response status code
	Status         string              `json:"status"`          // The HTTP response status message
	ContentType    string              `json:"content-type"`    // The HTTP content type
	ResponseHeader map[string][]string `json:"response-header"` // The HTTP response header
	Sha1           string              `json:"-"`               // SHA1 digest of the downloaded data
	SessionId      string              `json:"-"`               // The current session ID
}

// FileDownload has the metadata of a downloaded file and details
// about the download operation.
type FileDownload struct {
	Rch  chan error   // Handler response channel
	Md   Metadata     // Downloaded file metadata
	Info DownloadInfo // Download operation details
}

func NewFileDownload(md Metadata, info DownloadInfo) FileDownload {
	return FileDownload{
		Rch:  make(chan error, 1),
		Md:   md,
		Info: info,
	}
}

// Inspector is the interface implemented by artifact metadata
// extractors.
type Inspector interface {
	// Inspect extracts metadata from the given artifact and
	// populates the metadata structure, returning whether
	// the artifact was identified and no further examination
	// by other inspectors is required.
	Inspect(string, *Metadata, *DownloadInfo, *InspectionContext) (bool, error)
}

// inspectors has the list of inspectors to run on each downloaded
// artifact.
var inspectors = []Inspector{
	aptLegacyReleaseInspector{}, // apt legacy per-component Release file
	aptReleaseInspector{},       // apt Release/InRelease files
	aptPackagesInspector{},      // apt Packages.xz file
	debInspector{},              // deb packages
	whlInspector{},              // python wheels
	defaultInspector{},          // we don't know what this is
}

// InspectionContext contains session-specific contextual data for stateful
// analysis within a fetch session.
type InspectionContext struct {
	// releasePackages maps InRelease file digests to Packages.* file digests to metadata.
	releasePackages map[string]map[string]AptReleasePackages
	releaseLock     sync.Mutex

	// packagesEntries maps Packages.* file digests to package digest to metadata.
	packagesEntries map[string]map[string]AptPackagesEntry
	packagesLock    sync.Mutex
}

func NewInspectionContext() *InspectionContext {
	return &InspectionContext{
		releasePackages: make(map[string]map[string]AptReleasePackages, 16),
		packagesEntries: make(map[string]map[string]AptPackagesEntry, 160),
	}
}

// Run executes the registered inspectors for the artifact in the
// given directory, populating the metadata structure md.
func (ctx *InspectionContext) RunInspectors(dir string, md *Metadata, di *DownloadInfo) error {
	// detect file type
	filename := filepath.Join(dir, fmt.Sprintf("%s.bin", md.Sha1))
	f, err := mmap.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	mtype, err := mimetype.DetectReader(f)
	if err != nil {
		return err
	}

	md.Type = mtype.String()

	if len(di.ContentType) > 0 && !mtype.Is(di.ContentType) {
		log.Printf("warning: file type '%s' doesn't match content type '%s'", mtype.String(), di.ContentType)
	}

	// run metadata inspectors
	for _, ins := range inspectors {
		stop, err := ins.Inspect(filename, md, di, ctx)
		if err != nil {
			return err
		}
		if stop {
			break
		}
	}

	return nil
}

func (ctx *InspectionContext) AddReleasePackages(relDigest, digest string, p AptReleasePackages) {
	ctx.releaseLock.Lock()
	defer ctx.releaseLock.Unlock()

	if ctx.releasePackages[relDigest] == nil {
		ctx.releasePackages[relDigest] = make(map[string]AptReleasePackages, 16)
	}
	ctx.releasePackages[relDigest][digest] = p
	//log.Printf("apt releases file: %s %s", digest, p.Path)
}

func (ctx *InspectionContext) GetReleasePackages(digest string) (relDigest string, p AptReleasePackages, ok bool) {
	ctx.releaseLock.Lock()
	defer ctx.releaseLock.Unlock()

	for d, pkgs := range ctx.releasePackages {
		p, ok = pkgs[digest]
		if ok {
			relDigest = d
			return
		}
	}
	return
}

func (ctx *InspectionContext) AddPackagesEntry(pkgsDigest, digest string, e AptPackagesEntry) {
	ctx.packagesLock.Lock()
	defer ctx.packagesLock.Unlock()

	if ctx.packagesEntries[pkgsDigest] == nil {
		ctx.packagesEntries[pkgsDigest] = make(map[string]AptPackagesEntry)
	}
	ctx.packagesEntries[pkgsDigest][digest] = e
}

func (ctx *InspectionContext) GetPackagesEntry(digest string) (pkgsDigest string, e AptPackagesEntry, ok bool) {
	ctx.packagesLock.Lock()
	defer ctx.packagesLock.Unlock()

	for d, entries := range ctx.packagesEntries {
		e, ok = entries[digest]
		if ok {
			pkgsDigest = d
			return
		}
	}
	return
}

// findCallerInspector returns the name of the inspector that called
// this function's caller.
func findCallerInspector() string {
	pcs := make([]uintptr, 16)
	origin := "unknown"
	num := runtime.Callers(2, pcs)
	pcs = pcs[:num]
	frames := runtime.CallersFrames(pcs)

	for {
		frame, more := frames.Next()
		if !more {
			break
		}
		fname := frame.Function
		i := strings.LastIndexByte(fname, '/')
		if i < 0 {
			i = 0
		}
		if strings.HasSuffix(fname[i+1:], ".Inspect") {
			origin = strings.TrimSuffix(fname[i+1:], ".Inspect")
			break
		}
	}

	return origin
}

// defaultInspector is a fallback artifact inspector for unknown file
// formats.
type defaultInspector struct{}

func (ins defaultInspector) Inspect(filename string, md *Metadata, di *DownloadInfo, ctx *InspectionContext) (bool, error) {
	md.Annotate(Warning, "default.unknown", "unknown file format")
	return true, nil
}
