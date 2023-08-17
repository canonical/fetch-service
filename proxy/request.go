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
	//"fmt"
	"io"
	"net/http"
	"time"

	//"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/metadata"
)

// RequestHandler checks whether requests are authorized
type RequestHandler struct {
	ch         chan interface{}
	info       metadata.DownloadInfo
	body       io.ReadCloser // request body
	insTimeout time.Duration // artifact inspection timeout
}

func NewRequestHandler(req *http.Request, ch chan interface{}) (*RequestHandler, error) {
	sessionId := req.Header.Get(sessionIdHeader)

	h := &RequestHandler{
		ch: ch,
		info: metadata.DownloadInfo{
			StartTime: time.Now().UTC(),
			URL:       req.URL.String(),
			Address:   req.RemoteAddr,
			Method:    req.Method,
			UserAgent: req.Header.Get("User-Agent"),
			SessionId: sessionId,
		},
		body:       req.Body,
		insTimeout: 60 * time.Second,
	}

	return h, nil
}

// Read transfers data, computes digests and writes to a local copy of the file.
func (h *RequestHandler) Read(b []byte) (n int, err error) {
	n, err = h.body.Read(b)
	if err != nil && err != io.EOF {
		return
	}
	return
}

// Close finalizes the request.
func (h *RequestHandler) Close() error {
	res := h.body.Close()

	// update request information
	// TODO?

	return res

	/*
		fd := nil //auth.NewRequestURL(md, h.info)
		h.ch <- fd
		select {
		case err := <-fd.Rch:
			if err != nil {
				return fmt.Errorf("Error saving download data for asset %s: %v", sha1, err)
			}
		case <-time.After(h.insTimeout):
			logger.Errorf("inspection of artifact %s timed out", sha1)
		}

		return res
	*/
}
