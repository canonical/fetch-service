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
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/canonical/fetch-service/metadata"
)

// FileDownloadHandler creates local copies of downloaded files.
//
// This ReadCloser implementation computes sha1 and sha256 digests
// of downloaded contents and stores downloaded data and contextual
// metadata in the designated local file spool.
type FileDownloadHandler struct {
	ch       chan interface{}
	info     metadata.DownloadInfo
	size     int64         // streamed data size
	sha1     hash.Hash     // sha1 digest of streamed data
	sha256   hash.Hash     // sha256 digest of streamed data
	tempfile *os.File      // copy of streamed data
	body     io.ReadCloser // response body
	assetDir string        // file storage location
}

func NewFileDownloadHandler(resp *http.Response, spool string, ch chan interface{}) (*FileDownloadHandler, error) {
	sessionId := resp.Request.Header.Get(sessionIdHeader)

	tempfile, err := os.CreateTemp("", "fetch")
	if err != nil {
		return nil, err
	}

	req := resp.Request

	h := &FileDownloadHandler{
		ch: ch,
		info: metadata.DownloadInfo{
			StartTime:      time.Now(),
			URL:            req.URL.String(),
			Method:         req.Method,
			UserAgent:      req.Header.Get("User-Agent"),
			StatusCode:     resp.StatusCode,
			Status:         resp.Status,
			ContentType:    resp.Header.Get("Content-Type"),
			ResponseHeader: resp.Header,
			SessionId:      sessionId,
		},
		size:     0,
		sha1:     sha1.New(),
		sha256:   sha256.New(),
		tempfile: tempfile,
		body:     resp.Body,
		assetDir: filepath.Join(spool, sessionId, "assets"),
	}

	return h, nil
}

// Read transfers data, computes digests and writes to a local copy of the file.
func (h *FileDownloadHandler) Read(b []byte) (n int, err error) {
	n, err = h.body.Read(b)
	if err != nil && err != io.EOF {
		return
	}

	h.size += int64(n)
	h.sha1.Write(b[:n])
	h.sha256.Write(b[:n])

	size, e2 := h.tempfile.Write(b[:n])
	if e2 != nil {
		err = e2
		return
	}
	if size != n {
		err = fmt.Errorf("%s: short write", h.info.URL)
		return
	}

	return
}

// Close finalizes the transfer and writes metadata files to the spool.
func (h *FileDownloadHandler) Close() error {
	res := h.body.Close()
	if err := h.tempfile.Close(); err != nil {
		log.Printf("warning: %v", err)
	}

	sha1 := fmt.Sprintf("%x", h.sha1.Sum(nil))
	sha256 := fmt.Sprintf("%x", h.sha256.Sum(nil))

	// update download information
	h.info.EndTime = time.Now()
	h.info.Digest = sha1
	h.info.Size = h.size

	fi := metadata.FileInfo{
		Size:   h.size,
		Sha1:   sha1,
		Sha256: sha256,
	}

	dir := filepath.Join(h.assetDir, sha1)

	// save file data
	if err := saveFile(dir, h.tempfile.Name()); err != nil {
		return err
	}

	// save file metadata
	if err := saveFileMetadata(dir, fi); err != nil {
		return err
	}

	// save download metadata
	if err := saveDownloadMetadata(dir, h.info); err != nil {
		return err
	}

	h.ch <- h.info

	return res
}

func saveFile(dir, tempname string) error {
	dest := filepath.Join(dir, "data.bin")
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			if err := os.Rename(tempname, dest); err != nil {
				os.Remove(tempname)
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func saveFileMetadata(dir string, fi metadata.FileInfo) error {
	j, err := json.MarshalIndent(fi, "", "\t")
	if err != nil {
		return err
	}

	dest := filepath.Join(dir, "metadata.json")
	if err := ioutil.WriteFile(dest, j, 0644); err != nil {
		return err
	}

	return nil
}

func saveDownloadMetadata(dir string, info metadata.DownloadInfo) error {
	for i := 0; ; i++ {
		name := fmt.Sprintf("%08d.json", i)
		dest := filepath.Join(dir, name)
		if _, err := os.Stat(dest); err != nil {
			if os.IsNotExist(err) {
				// create file
				j, err := json.MarshalIndent(info, "", "\t")
				if err != nil {
					return err
				}
				if err := ioutil.WriteFile(dest, j, 0644); err != nil {
					return err
				}
			} else {
				return err
			}
			return nil
		}
	}
}
