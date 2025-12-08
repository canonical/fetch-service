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
	"fmt"
	"net"

	"github.com/canonical/fetch-service/service/fetchctl"
	"github.com/canonical/fetch-service/service/messages"
)

var createSessionCmd CreateSessionCmd

func init() {
	_, err := parser.AddCommand("create-session", "create a new fetch service session", "", &createSessionCmd)
	if err != nil {
		panic(err)
	}
}

type CreateSessionCmd struct {
	SessionID  string `long:"session-id" description:"Session ID of the newly created session"`
	Token      string `long:"token" description:"Session token of the newly created session"`
	Timeout    int    `long:"timeout" description:"Session timeout in seconds"`
	Permissive bool   `long:"permissive" description:"Create a permissive session"`
	Args       struct {
		Filename string `positional-arg-name:"filename"`
	} `positional-args:"yes"`
}

func (cmd *CreateSessionCmd) Execute(args []string) error {
	socket := fetchctlSocketPath()

	if err := checkSocket(socket); err != nil {
		return err
	}

	var content []byte
	var err error

	if len(cmd.Args.Filename) > 0 {
		content, err = readContent(cmd.Args.Filename)
		if err != nil {
			return err
		}
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	var mode string
	if cmd.Permissive {
		mode = "permissive"
	} else {
		mode = "strict"
	}

	createPayload := messages.CreateSessionPayload{
		SessionID:        cmd.SessionID,
		Token:            cmd.Token,
		Timeout:          cmd.Timeout,
		Mode:             mode,
		InspectorsConfig: content,
	}

	p, err := json.Marshal(createPayload)
	if err != nil {
		return err
	}

	request := fetchctl.OperationRequest{
		Operation: "create-session",
		Payload:   string(p),
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
