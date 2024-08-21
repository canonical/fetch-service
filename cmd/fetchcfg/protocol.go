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

package fetchcfg

import (
	"encoding/json"
	"net"

	"github.com/canonical/fetch-service/service/config"
)

func send(conn net.Conn, request config.OperationRequest) error {
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	_, err = conn.Write(data)
	return err
}

func receive(conn net.Conn, reply *config.OperationReply) error {
	data := make([]byte, 4096)
	n, err := conn.Read(data)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data[:n], reply); err != nil {
		return err
	}
	return nil
}
