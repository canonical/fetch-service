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

var updateConfigCmd UpdateConfigCmd

func init() {
	_, err := parser.AddCommand("update-config", "update the running service configuration", "", &updateConfigCmd)
	if err != nil {
		panic(err)
	}
}

type UpdateConfigCmd struct {
	Type         string `long:"type" choice:"acl" choice:"inspectors" description:"Type of configuration to update"`
	ValidateOnly bool   `long:"validate-only" description:"Validate the configuration and exit"`
	Args         struct {
		Filename string `positional-arg-name:"filename"`
	} `required:"yes" positional-args:"yes"`
}

func (cmd *UpdateConfigCmd) Execute(args []string) error {
	socket := fetchctlSocketPath()

	if err := checkSocket(socket); err != nil {
		return err
	}

	content, err := readContent(cmd.Args.Filename)
	if err != nil {
		return err
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	request := fetchctl.OperationRequest{
		Operation:    "update-config",
		Type:         cmd.Type,
		ValidateOnly: cmd.ValidateOnly,
		Payload:      string(content),
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

	fmt.Printf("%s %s\n", cmd.Type, reply.Message)

	return nil
}
