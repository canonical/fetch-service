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

package fetchctl

import (
	"encoding/json"
	"io"
	"net"

	"github.com/canonical/fetch-service/service/fetchctl"
)

func send(conn net.Conn, request fetchctl.OperationRequest) error {
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	_, err = conn.Write(data)
	return err
}

func receive(conn net.Conn, reply *fetchctl.OperationReply) error {
	data, err := io.ReadAll(conn)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, reply); err != nil {
		return err
	}
	return nil
}
