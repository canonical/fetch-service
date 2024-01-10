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

package proxy

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/service/messages"
)

// FileDownloadHandler creates local copies of downloaded files.
//
// This ReadCloser implementation computes sha1 and sha256 digests
// of downloaded contents and stores downloaded data and contextual
// metadata in the designated local file spool.
type FileDownloadHandler struct {
	ch       chan interface{}   // service message channel
	a        *metadata.Artefact // artefact metadata
	tempfile *os.File           // copy of streamed data
	body     io.ReadCloser      // response body
}

func NewFileDownloadHandler(resp *http.Response, a *metadata.Artefact, spool string, ch chan interface{}) (*FileDownloadHandler, error) {
	r := resp.Request
	sessionId := r.Header.Get(sessionIdHeader)
	insTimeout := 60 * time.Second // XXX: make this a configurable parameter
	assetDir := filepath.Join(spool, sessionId, "assets")

	tempfile, err := os.CreateTemp("", "fetch")
	if err != nil {
		return nil, err
	}

	a.Tempfile = tempfile.Name()
	a.AssetDir = assetDir

	if err := localDownload(resp, a, tempfile); err != nil {
		return nil, err
	}

	fd := messages.NewArtefactDownload(a)
	ch <- fd
	select {
	case err := <-fd.Rch:
		if err != nil {
			// XXX: if in strict mode, end this session
			return nil, err
		}
	case <-time.After(insTimeout):
		// XXX: if in strict mode, end this session
		return nil, fmt.Errorf("inspection of artefact %s timed out", a.Metadata.Sha256)
	}

	filename := fmt.Sprintf("%s.data", a.Metadata.Sha256)
	buffer, err := os.Open(filepath.Join(assetDir, filename))
	if err != nil {
		return nil, err
	}

	h := &FileDownloadHandler{
		ch:       ch,
		a:        a,
		tempfile: tempfile,
		body:     buffer,
	}

	return h, nil
}

// Read transfers data, computes digests and writes to a local copy of the file.
func (h *FileDownloadHandler) Read(b []byte) (n int, err error) {
	return h.body.Read(b)
}

// Close finalizes the transfer.
func (h *FileDownloadHandler) Close() error {
	res := h.body.Close()
	if err := h.tempfile.Close(); err != nil {
		logger.Warning(err)
	}

	// update download information
	h.a.CurrentDownload.EndTime = time.Now().UTC()

	return res
}

// localDownload stores the response body locally.
func localDownload(resp *http.Response, a *metadata.Artefact, tempfile io.WriteCloser) error {
	// download file for local buffering
	resp.Body = NewLocalDownloadHandler(resp, a)
	logger.Debugf("downloading %s...", resp.Request.URL)
	size, err := io.Copy(tempfile, resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if size != a.Metadata.Size {
		return fmt.Errorf("file size mismatch (%d, expected %d)", size, a.Metadata.Size)
	}

	a.CurrentDownload.Sha256 = a.Metadata.Sha256
	a.CurrentDownload.StatusCode = resp.StatusCode
	a.CurrentDownload.Status = resp.Status
	a.CurrentDownload.ContentType = resp.Header.Get("Content-Type")
	a.CurrentDownload.ResponseHeader = copyHeader(resp.Header)

	return nil
}

// LocalDownloadHandler computes digests during artefact download.
type LocalDownloadHandler struct {
	a      *metadata.Artefact // artefact metadata
	size   int64              // streamed data size
	sha1   hash.Hash          // sha1 digest of streamed data
	sha256 hash.Hash          // sha256 digest of streamed data
	body   io.ReadCloser      // response body
}

func NewLocalDownloadHandler(resp *http.Response, a *metadata.Artefact) *LocalDownloadHandler {
	logger.Debugf("create new proxy downloader")

	return &LocalDownloadHandler{
		a:      a,
		size:   0,
		sha1:   sha1.New(),
		sha256: sha256.New(),
		body:   resp.Body,
	}
}

func (h *LocalDownloadHandler) Read(b []byte) (n int, err error) {
	n, err = h.body.Read(b)

	h.size += int64(n)
	h.sha1.Write(b[:n])
	h.sha256.Write(b[:n])

	return n, err
}

func (h *LocalDownloadHandler) Close() error {
	res := h.body.Close()

	mver := fmt.Sprintf("%d.%d", metadata.MetadataVersionMajor, metadata.MetadataVersionMinor)
	h.a.Metadata.MetadataVersion = mver

	h.a.Metadata.Size = h.size
	h.a.Metadata.Sha1 = *(*metadata.Sha1Digest)(h.sha1.Sum(nil))
	h.a.Metadata.Sha256 = *(*metadata.Sha256Digest)(h.sha256.Sum(nil))

	return res
}
