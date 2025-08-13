// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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

package git

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/git/config"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
)

// The UploadPackInspector handles upload-pack requests. It recognizes
// the "ls-refs" and "fetch" commands.
type UploadPackInspector struct {
	heads map[string]map[string]string // head list stored by ls-refs per repository
	tags  map[string]map[string]string // tag list stored by ls-refs per repository

	config config.GitInspectorConfig
	lock   sync.Mutex
}

func NewUploadPackInspector(cfg config.GitInspectorConfig) *UploadPackInspector {
	return &UploadPackInspector{
		heads:  map[string]map[string]string{},
		tags:   map[string]map[string]string{},
		config: cfg,
	}
}

func (ins *UploadPackInspector) ID() string {
	return "git.upload-pack"
}

// InspectRequest verifies whether this is a valid upload-pack request. For
// it to succeed the following conditions must be satisfied:
//
//   - The "Git-Protocol" request header must be set to "version=2".
//   - The Content-Type header must be set to "application/x-git-upload-pack-request".
//   - The Accept header must be set to "application/x-git-upload-pack-result"
//   - The request URL must match a valid upload-pack pattern.
//   - The upload-pack command must be "ls-refs" or "fetch".
//   - If command is "fetch", it must want a single shallow ref.
func (ins *UploadPackInspector) InspectRequest(a RequestArtifact) error {
	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	slog := a.Logger()

	repo := fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
	repo, _ = strings.CutSuffix(repo, "/git-upload-pack")
	ins.initRepoState(repo)

	if !a.RequestHeaderContains("Content-Type", "application/x-git-upload-pack-request") {
		return nil
	}

	if !a.RequestHeaderContains("Accept", "application/x-git-upload-pack-result") {
		return nil
	}

	info, err := config.NewUploadPackUrlInfo(u, &ins.config, slog)
	if err != nil {
		info = &config.UploadPackUrlInfo{}
	}

	// We're now sure this is a git upload pack request

	// Set body to a new reader that inspects the git protocol so we can
	// examine the request body.
	proto := getGitProtocol(a)
	notes := Annotation{
		"server":     strings.SplitN(u.Host, ":", 2)[0],
		"repository": repo,
		"protocol":   proto,
		"project":    info.Project,
	}

	if proto != "version=2" {
		a.SetRequestUnknown(ins, "unsupported git protocol version").Annotate(Annotation{"proto": proto})
		return nil
	}

	// Read request body and get protocol messages
	var clientMsgs []string
	body, err := newUploadPackRequestHandler(ins.ID(), a.HTTPRequest(), &clientMsgs, slog)
	if err != nil {
		return fmt.Errorf("cannot handle upload-pack request: %w", err)
	}
	a.SetRequestBody(body)

	ins.inspectRequestCommand(a, clientMsgs, notes, slog)

	return nil
}

func (ins *UploadPackInspector) initRepoState(repo string) {
	ins.Lock()
	defer ins.Unlock()

	if _, ok := ins.heads[repo]; !ok {
		ins.heads[repo] = map[string]string{}
	}
	if _, ok := ins.tags[repo]; !ok {
		ins.tags[repo] = map[string]string{}
	}
}

func (ins *UploadPackInspector) inspectRequestCommand(a RequestArtifact, clientMsgs []string, notes Annotation, slog logger.Logger) {
	// Obtain the upload-pack command from the protocol messages
	command := ""
	notes.Add("client-request", clientMsgs)
	for _, msg := range clientMsgs {
		if strings.HasPrefix(msg, "command=") {
			command = msg[8:]
		}
	}
	notes.Add("command", command)
	slog.Debugf("git-upload-pack request command %s", command)

	switch command {
	case "ls-refs":
		a.SetRequestPending(ins, "valid URL for git upload-pack").Annotate(notes)
	case "fetch":
		// allow fetch only if shallow and single ref
		isShallow := false
		wantmap := map[string]struct{}{}
		wants := []string{}
		wantRefs := []string{}

		for _, msg := range clientMsgs {
			if strings.HasPrefix(msg, "deepen ") {
				isShallow = (msg == "deepen 1")
			} else if strings.HasPrefix(msg, "want ") {
				ref := msg[5:]
				if _, ok := wantmap[ref]; !ok { // don't store duplicate entries
					wantmap[ref] = struct{}{}
					wants = append(wants, ref)
				}
			} else if strings.HasPrefix(msg, "want-ref ") {
				wantRefs = append(wantRefs, msg[9:])
			}
		}

		notes.Add("is-shallow", isShallow)
		notes.Add("num-wants", len(wants)+len(wantRefs))
		if len(wants) > 0 {
			notes.Add("wants", wants)
		}
		if len(wantRefs) > 0 {
			notes.Add("want-refs", wantRefs)
		}

		if !isShallow {
			a.SetRequestRejected(ins, "fetch is only allowed with depth 1").Annotate(notes)
			return
		} else if len(wants) > 1 {
			a.SetRequestRejected(ins, "fetch is only allowed on a single ref").Annotate(notes)
			return
		}

		a.SetRequestPending(ins, "valid URL for git upload-pack").Annotate(notes)
	default:
		a.SetRequestRejected(ins, "only ls-refs and fetch commands are allowed").Annotate(notes)
	}

}

// InspectArtifact rejects upload-pack responses not conforming to the expected
// format for the "ls-ref" or "fetch" commands:
//
//   - The Content-Type header must be "application/x-git-upload-pack-result"
//   - If command is "ls-refs", we expect a text/plain response.
//   - If command is "fetch", we expect an application/octet-stream response, containing
//     the packfile for a single shallow ref.
//
// This inspector doesn't approve fetch artifacts and won't introspect into packfile
// contents, but it will leave annotations in case of a successful fetch operation.
// Approval is deferred to inspectors that examine specific types of git payloads.
func (ins *UploadPackInspector) InspectArtifact(f ArtifactReader, a ResponseArtifact) error {
	if a.ContentType() != "application/x-git-upload-pack-result" {
		return nil
	}

	repo, ok := a.RequestStringAnnotation(ins.ID(), "repository") // the upload-pack request repository
	if !ok {
		// this must have been set by the request inspector
		a.SetResponseRejected(ins, "repository not set during request inspection")
		return nil
	}

	ins.initRepoState(repo)

	vendor, _ := a.RequestStringAnnotation(ins.ID(), "server")

	return ins.inspectResponseCommand(f, a, repo, vendor, a.Logger())
}

func (ins *UploadPackInspector) inspectResponseCommand(f ArtifactReader, a ResponseArtifact, repo, vendor string, slog logger.Logger) error {
	command, ok := a.RequestAnnotation(ins.ID(), "command") // the upload-pack request command
	if !ok {
		// this must have been set by the request inspector
		a.SetResponseRejected(ins, "command not set during request inspection")
		return nil
	}

	notes := Annotation{}

	slog.Debugf("inspect git upload-pack artifact: command %q", command)
	// Supported upload-pack commands are 'ls-refs' and 'fetch'
	switch command {
	case "ls-refs":
		if err := ins.inspectLsRefsResponse(f, a, repo, vendor, notes, slog); err != nil {
			return err
		}

	case "fetch":
		if err := ins.inspectFetchResponse(f, a, repo, vendor, notes, slog); err != nil {
			return err
		}

	default:
		a.SetResponseRejected(ins, "only ls-refs and fetch commands are supported").Annotate(notes)
	}

	return nil
}

func (ins *UploadPackInspector) inspectLsRefsResponse(f ArtifactReader, a ResponseArtifact, repo, vendor string, notes Annotation, slog logger.Logger) error {
	if !a.MimetypeIs("text/plain") {
		a.SetResponseRejected(ins, "bad data type for ls-refs response")
		return nil
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.GitUploadPackLsRef,
		Name:        "git ls-refs response",
		Description: "Response to the git 'ls-refs' command",
		Vendor:      vendor,
	})

	msgs, err := decodeGitProtocol(f, slog)
	if err != nil {
		a.SetResponseRejected(ins, "cannot decode git protocol").Annotate(Annotation{"error-msg": err.Error()})
		return nil
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	refs := []string{}
	for _, msg := range msgs {
		refs = append(refs, strings.TrimSpace(msg))
	}
	notes.Add("server-response", refs)

	// store heads and tags in the inspector state for this repository
	for _, ref := range refs {
		p := strings.Split(ref, " ")
		if len(p) < 2 {
			continue
		}

		if tag, ok := strings.CutPrefix(p[1], "refs/tags/"); ok {
			ins.Lock()
			ins.tags[repo][tag] = p[0]
			ins.Unlock()
		} else if head, ok := strings.CutPrefix(p[1], "refs/heads/"); ok {
			ins.Lock()
			ins.heads[repo][head] = p[0]
			ins.Unlock()
		}
	}

	a.SetResponseApproved(ins, "git ls-refs response decoded").Annotate(notes)
	return nil
}

func (ins *UploadPackInspector) inspectFetchResponse(f ArtifactReader, a ResponseArtifact, repo, vendor string, notes Annotation, slog logger.Logger) error {
	if !a.MimetypeIs("application/octet-stream") && !a.MimetypeIs("text/plain") {
		a.SetResponseRejected(ins, "bad data type for fetch response")
		return nil
	}

	a.SetArtifactMetadata(ArtifactMetadata{
		Type:        mimetypes.GitUploadPackFetch,
		Name:        "git fetch response",
		Description: "Response to the git 'fetch' command",
		Vendor:      vendor,
	})

	if numWants, ok := a.RequestAnnotation(ins.ID(), "num-wants"); !ok || numWants != 1 {
		notes.Add("num-wants", numWants)
		a.SetResponseRejected(ins, "fetch is allowed only on a single ref").Annotate(notes)
		return nil
	}

	ins.Lock()
	notes.Add("heads", ins.heads[repo])
	notes.Add("tags", ins.tags[repo])
	ins.Unlock()

	msgs, err := decodeGitProtocol(f, slog)
	if err != nil {
		a.SetResponseRejected(ins, "cannot decode git protocol").Annotate(Annotation{"error-msg": err.Error()})
		return nil
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	if !ins.fetchResponseIsShallow(a, msgs, notes) {
		return nil
	}

	// Valid fetch command; unpack it and checkout the wanted ref so that git-based inspectors
	// can inspect its contents.
	ref := ins.getRefFromWants(a, notes)
	if ref == "" {
		return nil
	}

	// Unpack & checkout the contents into a shared location
	path, err := unpackAndCheckout(f, ref, a.CacheDir(), slog)
	if err != nil {
		a.SetResponseRejected(ins, "cannot unpack git objects").Annotate(Annotation{"error-msg": err.Error()})
		return nil
	}

	// At this point the git inspection was successful; annotate the path to the checked-out
	// files so that git-based inspectors can look into them.
	notes.Add("git-checkout-path", path)

	a.SetResponseUnknown(ins, "git fetch response is valid but content is unknown").Annotate(notes)
	return nil
}

func (ins *UploadPackInspector) fetchResponseIsShallow(a ResponseArtifact, msgs []string, notes Annotation) bool {
	serverMsgs := []string{}
	isShallow := false
	isUnshallow := false

	for _, msg := range msgs {
		if strings.HasPrefix(msg, "shallow ") {
			isShallow = true
		} else if strings.HasPrefix(msg, "unshallow ") {
			isUnshallow = true
		}
		serverMsgs = append(serverMsgs, strings.TrimSpace(msg))
	}
	notes.Add("server-response", serverMsgs)

	if !isShallow {
		a.SetResponseRejected(ins, "fetch is only allowed with depth 1").Annotate(notes)
		return false
	}

	if isUnshallow {
		a.SetResponseRejected(ins, "unshallow is not supported").Annotate(notes)
		return false
	}

	return true
}

func (ins *UploadPackInspector) getRefFromWants(a ResponseArtifact, notes Annotation) string {
	// Read "wants" information from the request annotation
	w, hasWants := a.RequestAnnotation(ins.ID(), "wants")
	wr, hasWantRefs := a.RequestAnnotation(ins.ID(), "want-refs")
	if !hasWants && !hasWantRefs {
		// this must have been set by the request inspection
		a.SetResponseRejected(ins, "cannot read request want/want-ref annotation").Annotate(notes)
		return ""
	}
	if !hasWants {
		// check out wanted-ref
		a.SetResponseRejected(ins, "want-refs handling not implemented yet").Annotate(notes)
		return ""
	}

	var wants []string
	if hasWants {
		var ok bool
		wants, ok = w.([]string)
		if !ok || len(wants) < 1 {
			a.SetResponseRejected(ins, "cannot read want annotation").Annotate(notes)
			return ""
		}
	}

	var wantRefs []string
	if hasWantRefs {
		var ok bool
		wantRefs, ok = wr.([]string)
		if !ok || len(wantRefs) < 1 {
			a.SetResponseRejected(ins, "cannot read want-ref annotation").Annotate(notes)
			return ""
		}
	}

	return wants[0]
}

func (ins *UploadPackInspector) Lock() {
	ins.lock.Lock()
}

func (ins *UploadPackInspector) Unlock() {
	ins.lock.Unlock()
}

// uploadPackRequestHandler is a ReaderCloser that decodes the body of
// a Git upload-pack request.
type uploadPackRequestHandler struct {
	body io.ReadCloser // request body
}

func newUploadPackRequestHandler(id string, req *http.Request, clientMsgs *[]string, slog logger.Logger) (*uploadPackRequestHandler, error) {
	isGzipped := req.Header.Get("Content-Encoding") == "gzip"

	// Handle gzip encoding
	if isGzipped {
		slog.Debugf("upload-pack request body is gzipped")
		gzipReader, err := gzip.NewReader(req.Body)
		if err != nil {
			return nil, fmt.Errorf("cannot create upload pack gzip decoder: %w", err)
		}

		req.Body = gzipReader
		req.Header.Del("Content-Encoding")
	}

	// Copy input buffer for decoding
	buf, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot read upload pack request body: %w", err)
	}
	req.ContentLength = int64(len(buf))

	// Decode protocol
	msgs, err := decodeGitProtocol(bytes.NewReader(buf), slog)
	if err != nil {
		return nil, fmt.Errorf("cannot decode upload pack request: %w", err)
	}

	// Remove line breaks from protocol messages
	for _, msg := range msgs {
		msg = strings.TrimSpace(msg)
		*clientMsgs = append(*clientMsgs, msg)
	}

	h := &uploadPackRequestHandler{
		body: io.NopCloser(bytes.NewReader(buf)),
	}

	return h, nil
}

func (h *uploadPackRequestHandler) Read(b []byte) (n int, err error) {
	n, err = h.body.Read(b)
	return
}

// Close finalizes the request.
func (h *uploadPackRequestHandler) Close() error {
	return h.body.Close()
}

func unpackAndCheckout(f ArtifactReader, ref string, cacheDir string, slog logger.Logger) (string, error) {
	// Unpack and checkout in temporary directory
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return "", err
	}

	dir, err := os.MkdirTemp(cacheDir, "git-")
	if err != nil {
		return "", err
	}

	slog.Debugf("unpack objects in %s", dir)
	if err := UnpackObjects(f, dir, slog); err != nil {
		return "", err
	}

	slog.Debugf("checkout ref %s in %s", ref, dir)
	if err := Checkout(dir, ref, slog); err != nil {
		return "", err
	}

	return dir, nil
}
