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

package git

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/inspectors/mimetypes"
	"github.com/canonical/fetch-service/logger"
)

// The UploadPackInspector handles upload-pack requests. It recognizes
// the "ls-refs" and "fetch" commands.
type UploadPackInspector struct {
	heads map[string]map[string]string // head list stored by ls-refs per repository
	tags  map[string]map[string]string // tag list stored by ls-refs per repository

	lock sync.Mutex
}

func NewUploadPackInspector() *UploadPackInspector {
	return &UploadPackInspector{
		heads: map[string]map[string]string{},
		tags:  map[string]map[string]string{},
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
func (ins *UploadPackInspector) InspectRequest(a RequestArtefact) error {
	proto := getGitProtocol(a)
	if proto != "version=2" {
		return nil
	}

	u, err := url.Parse(a.DownloadURL())
	if err != nil {
		return fmt.Errorf("cannot parse URL: %s", err)
	}

	repo := fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
	repo, _ = strings.CutSuffix(repo, "/git-upload-pack")
	ins.Lock()
	if _, ok := ins.heads[repo]; !ok {
		ins.heads[repo] = map[string]string{}
	}
	if _, ok := ins.tags[repo]; !ok {
		ins.tags[repo] = map[string]string{}
	}
	ins.Unlock()

	content_type, ok := a.RequestHeader("Content-Type")
	if !ok || len(content_type) < 1 || content_type[0] != "application/x-git-upload-pack-request" {
		return nil // we don't recognize this request
	}

	accept, ok := a.RequestHeader("Accept")
	if !ok || len(accept) < 1 || accept[0] != "application/x-git-upload-pack-result" {
		return nil // we don't recognize this request
	}

	// FIXME: adjust according to internal git repository url format
	info, err := newUploadPackUrlInfo(u)
	if err != nil {
		info = &uploadPackUrlInfo{}
	}

	// We're now sure this is a git upload pack request

	// Set body to a new reader that inspects the git protocol so we can
	// examine the request body.
	notes := Annotation{
		"repository": repo,
		"protocol":   proto,
		"project":    info.project,
	}

	// Read request body and get protocol messages
	var client_msgs []string
	body, err := newUploadPackRequestHandler(ins.ID(), a.HTTPRequest(), &client_msgs)
	if err != nil {
		return fmt.Errorf("cannot handle upload-pack request: %w", err)
	}
	a.SetRequestBody(body)

	// Obtain the upload-pack command from the protocol messages
	command := ""
	notes.Add("client-request", client_msgs)
	for _, msg := range client_msgs {
		if strings.HasPrefix(msg, "command=") {
			command = msg[8:]
		}
	}
	notes.Add("command", command)
	logger.Debugf("git-upload-pack request command %s", command)

	// Special actions for commands
	switch command {
	case "ls-refs":
		// check if url matches
		if info.project == "" {
			return nil
		}
	case "fetch":
		// allow fetch only if shallow and single ref
		isShallow := false
		wants := []string{}
		want_refs := []string{}

		for _, msg := range client_msgs {
			if strings.HasPrefix(msg, "deepen ") {
				isShallow = (msg == "deepen 1")
			} else if strings.HasPrefix(msg, "want ") {
				wants = append(wants, msg[5:])
			} else if strings.HasPrefix(msg, "want-ref ") {
				want_refs = append(want_refs, msg[9:])
			}
		}

		notes.Add("is-shallow", isShallow)
		notes.Add("num-wants", len(wants)+len(want_refs))
		if len(wants) > 0 {
			notes.Add("wants", wants)
		}
		if len(want_refs) > 0 {
			notes.Add("want-refs", want_refs)
		}

		if !isShallow {
			a.SetRequestRejected(ins, "fetch is only allowed with depth 1").Annotate(notes)
			return nil
		} else if len(wants) > 1 {
			a.SetRequestRejected(ins, "fetch is only allowed on a single ref").Annotate(notes)
			return nil
		}
	default:
		a.SetRequestRejected(ins, "only ls-refs and fetch commands are allowed").Annotate(notes)
		return nil
	}

	a.SetRequestPending(ins, "valid URL for git upload-pack").Annotate(notes)
	return nil
}

// InspectArtefact rejects upload-pack responses not conforming to the expected
// format for the "ls-ref" or "fetch" commands:
//
//   - The Content-Type header must be "application/x-git-upload-pack-result"
//   - If command is "ls-refs", we expect a text/plain response.
//   - If command is "fetch", we expect an application/octet-stream response, containing
//     the packfile for a single shallow ref.
//
// This inspector doesn't approve fetch artefacts and won't introspect into packfile
// contents, but it will leave annotations in case of a successful fetch operation.
// Approval is deferred to inspectors that examine specific types of git payloads.
func (ins *UploadPackInspector) InspectArtefact(f ArtefactFile, a ResponseArtefact) error {

	if a.ContentType() != "application/x-git-upload-pack-result" {
		return nil
	}

	command, ok := a.RequestAnnotation(ins.ID(), "command") // the upload-pack request command
	if !ok {
		// this must have been set by the request inspector
		return errors.New("cannot read request command annotation")
	}
	notes := Annotation{}

	logger.Debugf("inspect git upload-pack artefact: command %q", command)

	repo, ok := a.RequestStringAnnotation(ins.ID(), "repository") // the upload-pack request command
	if !ok {
		// this must have been set by the request inspector
		return errors.New("cannot read repository annotation")
	}
	ins.Lock()
	if _, ok := ins.heads[repo]; !ok {
		ins.heads[repo] = map[string]string{}
	}
	if _, ok := ins.tags[repo]; !ok {
		ins.tags[repo] = map[string]string{}
	}
	ins.Unlock()

	// Supported upload-pack commands are 'ls-refs' and 'fetch'
	switch command {
	case "ls-refs":
		if !a.MimetypeIs("text/plain") {
			a.SetResponseRejected(ins, "bad data type for ls-refs response")
			return nil
		}

		a.SetArtefactMetadata(ArtefactMetadata{Type: mimetypes.GitUploadPackLsRef})

		msgs, err := decodeGitProtocol(f)
		if err != nil {
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

		a.SetResponseApproved(ins,
			"git ls-refs response decoded").Annotate(notes)

	case "fetch":
		if !a.MimetypeIs("application/octet-stream") {
			a.SetResponseRejected(ins, "bad data type for fetch response")
			return nil
		}

		a.SetArtefactMetadata(ArtefactMetadata{Type: mimetypes.GitUploadPackFetch})

		if numWants, ok := a.RequestAnnotation(ins.ID(), "num-wants"); !ok || numWants != 1 {
			notes.Add("num-wants", numWants)
			a.SetResponseRejected(ins,
				"fetch is allowed only on a single ref").Annotate(notes)
			return nil
		}
		ins.Lock()
		notes.Add("heads", ins.heads[repo])
		notes.Add("tags", ins.tags[repo])
		ins.Unlock()

		msgs, err := decodeGitProtocol(f)
		if err != nil {
			return fmt.Errorf("error decoding git protocol: %w", err)
		}

		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return err
		}

		server_msgs := []string{}
		isShallow := false
		for _, msg := range msgs {
			if strings.HasPrefix(msg, "shallow ") {
				isShallow = true
			}
			server_msgs = append(server_msgs, strings.TrimSpace(msg))
		}
		notes.Add("server-response", server_msgs)

		if !isShallow {
			a.SetResponseRejected(ins,
				"fetch is only allowed with depth 1").Annotate(notes)
			return nil
		}

		if numWants, ok := a.RequestAnnotation(ins.ID(), "num-wants"); !ok || numWants != 1 {
			notes.Add("num-wants", numWants)
			a.SetResponseRejected(ins,
				"fetch is allowed only on a single ref").Annotate(notes)
			return nil
		}

		notes.Add("server-response", server_msgs)
		a.SetResponseUnknown(ins, "git fetch response is valid but content is unknown").Annotate(notes)

	default:
		a.SetResponseRejected(ins, "only ls-refs and fetch commands are supported").Annotate(notes)
	}

	return nil
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

func newUploadPackRequestHandler(id string, req *http.Request, client_msgs *[]string) (*uploadPackRequestHandler, error) {
	isGzipped := req.Header.Get("Content-Encoding") == "gzip"

	// Handle gzip encoding
	if isGzipped {
		logger.Debugf("upload-pack request body is gzipped")
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
	msgs, err := decodeGitProtocol(bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("cannot decode upload pack request: %w", err)
	}

	// Remove line breaks from protocol messages
	for _, msg := range msgs {
		msg = strings.TrimSpace(msg)
		*client_msgs = append(*client_msgs, msg)
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
