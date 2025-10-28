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

package proxy

import (
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/service/messages"
)

// Time to wait before switching to background download.
const bgTimeout = 5 * time.Second

// FileDownloadHandler creates local copies of downloaded files.
//
// This ReadCloser implementation computes sha1 and sha256 digests
// of downloaded contents and stores downloaded data and contextual
// metadata in the designated local file spool.
type FileDownloadHandler struct {
	ch   chan interface{}   // service message channel
	a    *metadata.Artifact // artifact metadata
	body io.ReadCloser      // response body
}

func NewFileDownloadHandler(resp *http.Response, a *metadata.Artifact, spool string, ch chan interface{}) (*FileDownloadHandler, error) {
	r := resp.Request
	sessionID, err := getSessionIdHeader(r)
	if err != nil {
		return nil, err
	}

	insTimeout := 60 * time.Second // XXX: make this a configurable parameter
	assetDir := filepath.Join(spool, sessionID, "assets")
	cacheDir := filepath.Join(spool, sessionID, "cache")

	tempfile, err := os.CreateTemp("", "artifact-")
	if err != nil {
		return nil, err
	}

	a.Tempfile = tempfile.Name()
	a.AssetDir = assetDir
	a.SessionCacheDir = cacheDir
	slog := a.Logger()

	// Create a pipe to pass the response body. The file download handler
	// is only used if the response from the server is 200.
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("cannot create body data pipe: %w", err)
	}

	slog.Debugf("original server response: %+v\n", resp)
	dch := make(chan error, 1)

	var bgDownload atomic.Bool // Set when the file is downloaded or inspected in background.

	// Execute the local download and inspection in parallel. This will feed the
	// user download pipe with the locally buffered data, and block until the
	// original client downloads the file.
	go func() {
		// Start downloading the data. If it takes less than 5 seconds, return
		// immediately. Otherwise return 200 OK and send the data when it's
		// downloaded and passed inspection, or close the reader if it was
		// rejected.
		defer pw.Close()

		if err = localDownload(resp, a, tempfile); err != nil {
			slog.Warningf("local download error: %s", err)
			dch <- err
			if bgDownload.Load() {
				pr.Close()
			}
			return
		}

		slog.Debug("request inspection")
		respIns := messages.NewResponseInspection(a)
		ch <- respIns
		select {
		case err := <-respIns.Rch: // Wait for the inspection to finish and get its result.
			slog.Debugf("local inspection result: %v", err)
			if err != nil {
				dch <- err
				if bgDownload.Load() {
					pr.Close()
				}
				return
			}
		case <-time.After(insTimeout): // Inspection took too long, something wrong happened.
			slog.Warning("inspection request timeout")
			dch <- fmt.Errorf("inspection of artifact %s timed out", a.Metadata.Sha256)
			if bgDownload.Load() {
				pr.Close()
			}
			return
		}

		// Open the asset file and pipe the contents to the response body
		// being read by the original client.
		filename := fmt.Sprintf("%s.data", a.Metadata.Sha256)
		buffer, err := os.Open(filepath.Join(assetDir, filename))
		if err != nil {
			dch <- fmt.Errorf("cannot open asset file: %w", err)
			if bgDownload.Load() {
				pr.Close()
			}
			return
		}
		defer buffer.Close()

		dch <- nil // Download completed successfully.
		_, err = io.Copy(pw, buffer)
		if err != nil {
			slog.Warningf("response body copy error: %v", err)
		}
	}()

	select {
	case err := <-dch:
		// We have a response in less than 5 seconds, return it.
		if err != nil {
			// Download or inspection failed, return the error.
			return nil, err
		}
		// Download succeeded, return its data.
	case <-time.After(bgTimeout):
		// This is a long download, keep downloading it but return an HTTP header
		// so the client will not time out waiting for a response.
		bgDownload.Store(true)
		slog.Info("switch to background artifact download and inspection")
	}

	h := &FileDownloadHandler{
		ch:   ch,
		a:    a,
		body: pr, // The client will get the body from the reading side of the pipe.
	}

	return h, nil
}

// Read transfers data, computes digests and writes to a local copy of the file.
func (h *FileDownloadHandler) Read(b []byte) (int, error) {
	return h.body.Read(b)
}

// Close finalizes the transfer.
func (h *FileDownloadHandler) Close() error {
	return h.body.Close()
}

// Extract and validate the session ID from the request header
func getSessionIdHeader(r *http.Request) (string, error) {
	id := r.Header.Get(sessionIDHeader)
	if id == "" {
		return "", errors.New("session ID cannot be empty")
	}

	for _, c := range id {
		if (c < 'a' || 'z' < c) && (c < '0' || '9' < c) {
			return "", fmt.Errorf("invalid session ID: %q", id)
		}
	}

	return id, nil
}

// localDownload stores the response body locally.
func localDownload(resp *http.Response, a *metadata.Artifact, tempfile io.WriteCloser) error {
	slog := a.Logger()

	// download file for local buffering
	slog.Debugf("downloading %s", resp.Request.URL)
	resp.Body = NewLocalDownloadHandler(resp, a)
	size, err := io.Copy(tempfile, resp.Body)
	if err != nil {
		return err
	}

	if err := resp.Body.Close(); err != nil {
		return err
	}
	slog.Debugf("finished downloading %s", resp.Request.URL)

	if size != a.Metadata.Size {
		return fmt.Errorf("file size mismatch (%d, expected %d)", size, a.Metadata.Size)
	}

	a.CurrentDownload.Sha256 = a.Metadata.Sha256
	a.CurrentDownload.StatusCode = resp.StatusCode
	a.CurrentDownload.Status = resp.Status
	a.CurrentDownload.ContentType = resp.Header.Get("Content-Type")
	a.CurrentDownload.ResponseHeader = copyHTTPHeader(resp.Header)

	return nil
}

// LocalDownloadHandler computes digests during artifact download.
type LocalDownloadHandler struct {
	a      *metadata.Artifact // artifact metadata
	size   int64              // streamed data size
	sha1   hash.Hash          // sha1 digest of streamed data
	sha256 hash.Hash          // sha256 digest of streamed data
	body   io.ReadCloser      // response body
}

func NewLocalDownloadHandler(resp *http.Response, a *metadata.Artifact) *LocalDownloadHandler {
	logger.Debugf("proxy: create new proxy downloader")

	return &LocalDownloadHandler{
		a:      a,
		size:   0,
		sha1:   sha1.New(),
		sha256: sha256.New(),
		body:   resp.Body,
	}
}

func (h *LocalDownloadHandler) Read(b []byte) (int, error) {
	n, err := h.body.Read(b)

	h.size += int64(n)
	h.sha1.Write(b[:n])
	h.sha256.Write(b[:n])

	return n, err
}

func (h *LocalDownloadHandler) Close() error {
	res := h.body.Close()

	h.a.Metadata.Size = h.size
	h.a.Metadata.Sha1 = *(*digests.Sha1Digest)(h.sha1.Sum(nil))
	h.a.Metadata.Sha256 = *(*digests.Sha256Digest)(h.sha256.Sum(nil))

	return res
}
