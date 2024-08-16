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
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
)

var (
	shortHelp = "Fetch service configuration tool"
	longHelp  = `
Fetchcfg is a tool to update the configuration of the running
fetch service.`
)

var opts struct {
	// Show version
	Version bool `long:"version" description:"Display the program version and exit"`
}

func Run() int {
	p := parser()

	_, err := p.ParseArgs(os.Args[1:])
	if err != nil {
		printf("error: %v", err)
		return 1
	}

	if opts.Version {
		printf("fetchcfg %s\n", Version)
		return 0
	}

	return 0
}

var printf = printfImpl

func printfImpl(format string, a ...any) {
	fmt.Printf(format, a...)
}

func parser() *flags.Parser {
	p := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash|flags.PassAfterNonOption)
	p.ShortDescription = shortHelp
	p.LongDescription = longHelp
	return p
}

// func main() {
// 	os.Exit(Run())
// }
