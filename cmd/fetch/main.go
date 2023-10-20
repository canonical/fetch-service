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
	"os/signal"
	"syscall"

	"github.com/jessevdk/go-flags"

	"github.com/canonical/fetch-service/logger"
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

	// Path to the local spool containing downloaded files and extracted metadata.
	Spool string `long:"spool" description:"Path to downloaded dependencies" default:"/var/lib/fetch"`

	// Set the verbosity level
	Verbosity string `long:"verbosity" description:"Verbosity level" choice:"debug"`

	// Enable permissive mode
	PermissiveMode bool `long:"permissive-mode" description:"Don't reject invalid downloads"`
}

func main() {
	p := parser()

	_, err := p.ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Printf("error: %v", err)
		os.Exit(1)
	}

	opt := service.Options{
		Port:           opts.Port,
		Spool:          opts.Spool,
		PermissiveMode: opts.PermissiveMode,
	}

	lv := logger.InfoLevel
	if opts.Verbosity == "debug" {
		lv = logger.DebugLevel
	}

	logger.Init(lv)
	defer logger.Close()

	logger.Debug("Running in debug mode")

	svc := service.New(&opt)
	if err := svc.Start(); err != nil {
		logger.Fatalf("Cannot start service: %s", err)
	}

	// Shut down gracefully if terminated.
	cs := make(chan os.Signal, 1)
	signal.Notify(cs, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

loop:
	for {
		select {
		case sig := <-cs:
			logger.Infof("Exiting on %s signal.\n", sig)
			break loop
		case <-svc.Dying():
			// something called Stop()
			logger.Info("Server exiting!")
			break loop

			// TODO: add watchdog
		}
	}

	if err := svc.Stop(); err != nil {
		logger.Fatalf("error: %s", err)
	}
}

func parser() *flags.Parser {
	p := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash|flags.PassAfterNonOption)
	p.ShortDescription = shortHelp
	p.LongDescription = longHelp
	return p
}
