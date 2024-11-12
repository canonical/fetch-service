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
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/jessevdk/go-flags"

	ctlserver "github.com/canonical/fetch-service/service/fetchctl"
)

var (
	shortHelp = "Fetch service configuration tool"
	longHelp  = `
Fetchcfg is a tool to update the configuration of the running
fetch service.`
)

var (
	opts struct {
	}

	parser = flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash|flags.PassAfterNonOption)
)

var (
	fetchctlSocketPath = ctlserver.SocketPath
)

func Run() int {
	parser.ShortDescription = shortHelp
	parser.LongDescription = longHelp

	_, err := parser.ParseArgs(os.Args[1:])
	if err != nil {
		printf("error: %s\n", err)
		return 1
	}

	return 0
}

var printf = printfImpl

func printfImpl(format string, a ...any) {
	fmt.Printf(format, a...)
}

func checkSocket(filename string) error {
	if _, err := os.Stat(filename); err != nil {
		return errors.New("cannot access socket, is the service running?")
	}
	return nil
}

func readContent(filename string) ([]byte, error) {
	var content []byte
	var err error
	if filename == "-" {
		content, err = io.ReadAll(os.Stdin)
	} else {
		content, err = os.ReadFile(filename)
	}
	return content, err
}

// func main() {
// 	os.Exit(Run())
// }
