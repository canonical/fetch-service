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
	"fmt"
	"net"

	"github.com/canonical/fetch-service/service/fetchctl"
)

type ListArtifactsCmd struct {
	SessionId string `long:"session-id" required:"true" description:"ID of the session holding the artifacts to list"`
}

func (cmd *ListArtifactsCmd) Execute(args []string) error {
	socket := fetchctlSocketPath()

	if err := checkSocket(socket); err != nil {
		return err
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return err
	}
	defer conn.Close()

	request := fetchctl.OperationRequest{
		Operation: "list-artifacts",
		Payload:   cmd.SessionId,
	}
	err = send(conn, request)
	if err != nil {
		return fmt.Errorf("cannot send request: %s", err)
	}

	var reply fetchctl.OperationReply
	if err := receive(conn, &reply); err != nil {
		return fmt.Errorf("cannot read reply: %s", err)
	}

	if reply.Result != "ok" {
		return fmt.Errorf("%s", reply.Message)
	}

	fmt.Printf("%s\n", reply.Message)

	return nil
}

var listArtifactsCmd ListArtifactsCmd

func init() {
	_, _ = parser.AddCommand("list-artifacts", "list the session artifacts", "", &listArtifactsCmd)
}
