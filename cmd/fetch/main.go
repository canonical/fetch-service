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

package main

import (
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"

	"github.com/canonical/fetch-service/service"
)

var (
	shortHelp = "Network access helper for craft tools"
	longHelp  = `
The fetch service is a tool to mediate network access when executing
craft tools.`
)

var opts struct {
	// The TCP port the service will listen on.
	Port int `short:"p" long:"port" description:"Port number" default:"9988"`
}

func main() {
	p := parser()

	_, err := p.ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Printf("error: %v", err)
		os.Exit(1)
	}

	opt := service.Options{Port: opts.Port}

	svc := service.New(&opt)
	svc.Start()
}

func parser() *flags.Parser {
	p := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash|flags.PassAfterNonOption)
	p.ShortDescription = shortHelp
	p.LongDescription = longHelp
	return p
}
