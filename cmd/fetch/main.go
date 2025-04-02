// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2025 Canonical Ltd.
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
	"path/filepath"
	"syscall"

	"github.com/jessevdk/go-flags"

	"github.com/canonical/fetch-service/cmd/fetchctl"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/profile"
	"github.com/canonical/fetch-service/service"
	"github.com/canonical/fetch-service/version"
)

var (
	shortHelp = "Network access helper for craft tools"
	longHelp  = `
The fetch service is a tool to mediate network access when executing
craft tools.`
)

type CmdlineOptions struct {
	// Enable profiling API
	Profile bool `long:"profile" description:"Enable profiling"`

	// Profiling API port number
	ProfilePort int `long:"profile-port" description:"Profiling API port number" default:"6060"`

	// The TCP port the control API server will listen on.
	ControlPort int `long:"control-port" description:"Control port number" default:"9999"`

	// The TCP port the proxy server will listen on.
	ProxyPort int `short:"p" long:"proxy-port" description:"Proxy port number" default:"9988"`

	// Path to the configuration files.
	Config string `long:"config" description:"Path to the directory containing configuration files"`

	// Path to the local spool containing downloaded files and extracted metadata.
	Spool string `long:"spool" description:"Path to downloaded dependencies"`

	// Set the verbosity level
	Verbosity string `long:"verbosity" description:"Verbosity level" choice:"debug"`

	// Enable permissive mode
	PermissiveMode bool `long:"permissive-mode" description:"Allow sessions to accept rejected artifacts"`

	// Show version
	Version bool `long:"version" description:"Display the program version and exit"`

	// Certificate for the MITM proxy
	CertPath string `long:"cert" description:"The path to a file containing the HTTPS proxy certificate"`

	// Private key for the MITM proxy
	KeyPath string `long:"key" description:"The path to a file containing the HTTPS proxy private key"`

	// Auto-shutdown the service when idle
	IdleShutdown int `long:"idle-shutdown" description:"Time in seconds to auto-shutdown if idle"`

	// Specify the path to the log file
	LogFile string `long:"log-file" description:"Log to this file instead of standard out"`
}

var opts CmdlineOptions

func Run() int {
	p := parser()

	_, err := p.ParseArgs(os.Args[1:])
	if err != nil {
		printf("error: %v", err)
		return 1
	}

	if opts.Version {
		printf("fetch %s\n", version.Version)
		return 0
	}

	lv := logger.InfoLevel
	if opts.Verbosity == "debug" {
		lv = logger.DebugLevel
	}
	err = logger.Init(lv, opts.LogFile)
	if err != nil {
		printf("error: %v", err)
		return 1
	}
	defer logger.Close()

	logger.Infof("Version %s", version.Version)
	logger.Debug("Running in debug mode")

	// Start continuous profiling server
	pp := profile.NewProfiler(opts.ProfilePort)
	if opts.Profile {
		pp.Start()
	}

	svcOpts := getServiceOptions(opts)

	svc, err := service.New(svcOpts)
	if err != nil {
		logger.Fatalf("Cannot create service: %s", err)
	}

	if err := svc.Start(); err != nil {
		logger.Fatalf("Cannot start service: %s", err)
	}

	// Shut down gracefully if terminated.
	cs := make(chan os.Signal, 1)
	signal.Notify(cs, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	status := 0
loop:
	for {
		select {
		case sig := <-cs:
			logger.Infof("Exiting on %s signal.\n", sig)
			if sysSig, ok := sig.(syscall.Signal); ok {
				status = 128 + int(sysSig)
			} else {
				status = 128
			}
			break loop

		case <-svc.Dying():
			if err := svc.Err(); err != nil {
				logger.Errorf("Server error: %s", err)
				status = 2
			} else {
				// something called Stop()
				logger.Info("Server exiting!")
			}
			break loop

		case <-pp.Dying():
			// profiling server died
			logger.Errorf("Profiling error: %s", pp.Err())
			status = 3
		}
	}

	if err := svc.Stop(); err != nil {
		logger.Fatalf("error: %s", err)
	}

	return status
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

// Get the value of an option that can be provided by the command line
// depending on the running environment and whether a command line option
// was provided by the user.
func getOptionOrDefault(cmdlineValue, snapDefault, nonsnapDefault string) string {
	if cmdlineValue != "" {
		// Value provided on the command line; use it
		return cmdlineValue
	}
	if service.RunningFromSnap() {
		return os.ExpandEnv(snapDefault)
	}
	return nonsnapDefault
}

func getServiceOptions(opts CmdlineOptions) *service.Options {
	configDir := getOptionOrDefault(opts.Config, service.SnapConfigDir, service.NonSnapConfigDir)
	spoolDir := getOptionOrDefault(opts.Spool, service.SnapSpoolDir, service.NonSnapSpoolDir)

	// Start the fetch service
	return &service.Options{
		ControlPort:    opts.ControlPort,
		ProxyPort:      opts.ProxyPort,
		Config:         configDir,
		Spool:          spoolDir,
		PermissiveMode: opts.PermissiveMode,
		CertPath:       opts.CertPath,
		KeyPath:        opts.KeyPath,
		IdleShutdown:   opts.IdleShutdown,
	}

}

func main() {
	cmd := filepath.Base(os.Args[0])
	if cmd == "fetchctl" || cmd == "fetch-service.fetchctl" {
		os.Exit(fetchctl.Run())
	}

	os.Exit(Run())
}
