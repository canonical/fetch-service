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

package common

import (
	"io"
)

type ReadAtSeeker interface {
	io.ReadSeeker
	io.ReaderAt
	Len() int
}

type InspectorAPI interface {
}

/*
func GetInspectorAPI(sessionId, insName string, ch chan interface{}) (InspectorAPI, error) {
	rch := make(chan InspectorAPI)
	req := InspectorAPIRequest{
		Rch:       rch,
		SessionId: sessionId,
		InsName:   insName,
	}

	ch <- req
	res := <-req.Rch
	if res == nil {
		return nil, fmt.Errorf("cannot obtain %s API", insName)
	}

	return res, nil
}
*/

type SessionDetails interface {
	GetInspectorAPI(name string) (InspectorAPI, error)
}

/*
type InspectorAPIRequest struct {
	Rch       chan InspectorAPI // Handler response channel
	SessionId string
	InsName   string
}
*/
