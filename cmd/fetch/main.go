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

type Opts struct {
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

func Run() int {
	opts := Opts{}
	p := parser(&opts)

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

	svc_opt := getServiceOptions(&opts)

	// Start the fetch service

	svc, err := service.New(&svc_opt)
	if err != nil {
		logger.Fatalf("Cannot create service: %s", err)
	}

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

		case <-pp.Dying():
			// profiling server died
			logger.Errorf("Profiling error: %s", pp.Err())

			// TODO: add watchdog
		}
	}

	if err := svc.Stop(); err != nil {
		logger.Fatalf("error: %s", err)
	}

	return 0
}

var printf = printfImpl

func printfImpl(format string, a ...any) {
	fmt.Printf(format, a...)
}

func parser(opts *Opts) *flags.Parser {
	p := flags.NewParser(opts, flags.HelpFlag|flags.PassDoubleDash|flags.PassAfterNonOption)
	p.ShortDescription = shortHelp
	p.LongDescription = longHelp
	return p
}

func is_snap() bool {
	return os.Getenv("SNAP_NAME") == "fetch-service" && os.Getenv("SNAP") != ""
}

// Get the "real" value of an option that can be provided by the command line (cmdline_value),
// and has different defaults depending on whether the fetch-service is running as a snap (snap_default)
// or not (nonsnap_default).
// For snap_default, environment variables (like SNAP_DATA and SNAP_COMMON) are expanded.
func getOptionOrDefault(cmdline_value, snap_default, nonsnap_default string) string {
	if cmdline_value != "" {
		// Value provided on the command line; use it
		return cmdline_value
	}

	if is_snap() {
		return os.ExpandEnv(snap_default)
	}
	return nonsnap_default
}

func getServiceOptions(opts *Opts) service.Options {
	var config = getOptionOrDefault(opts.Config, "${SNAP_DATA}/conf", "/etc/fetch")
	var spool = getOptionOrDefault(opts.Spool, "${SNAP_COMMON}/spool", "/var/lib/fetch")

	return service.Options{
		ControlPort:    opts.ControlPort,
		ProxyPort:      opts.ProxyPort,
		Config:         config,
		Spool:          spool,
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
