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
	ch         chan interface{}   // service messaging channel
	a          *metadata.Artefact // artefact metadata
	body       io.ReadCloser      // request body
	insTimeout time.Duration      // artifact inspection timeout
}

func NewRequestHandler(req *http.Request, a *metadata.Artefact, ch chan interface{}) (*RequestHandler, error) {
	h := &RequestHandler{
		ch:         ch,
		a:          a,
		body:       req.Body,
		insTimeout: 60 * time.Second,
	}

	return h, nil
}

func (h *RequestHandler) Read(b []byte) (n int, err error) {
	n, err = h.body.Read(b)
	return
}

// Close finalizes the request.
func (h *RequestHandler) Close() error {
	res := h.body.Close()

	// update request information
	// TODO?

	return res
}
