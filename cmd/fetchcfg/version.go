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
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/canonical/fetch-service/service/config"
)

type VersionCmd struct {
}

var configSocketPath = config.SocketPath

func (cmd *VersionCmd) Execute(args []string) error {
	socket := configSocketPath()

	if _, err := os.Stat(socket); err != nil {
		return errors.New("cannot access socket, is the service running?")
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return err
	}
	defer conn.Close()

	request := config.OperationRequest{Operation: "version"}
	err = send(conn, request)
	if err != nil {
		return fmt.Errorf("cannot send request: %s", err)
	}

	var reply config.OperationReply
	if err := receive(conn, &reply); err != nil {
		return fmt.Errorf("cannot read reply: %s", err)
	}

	if reply.Result != "ok" {
		return fmt.Errorf("cannot obtain service version: %s", reply.Message)
	}

	fmt.Printf("%s\n", reply.Message)

	return nil
}

var versionCmd VersionCmd

func init() {
	_, _ = parser.AddCommand("version", "check the Fetch Service version", "long description", &versionCmd)
}
