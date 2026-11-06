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
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/jessevdk/go-flags"
	"golang.org/x/sys/unix"

	"github.com/canonical/fetch-service/cmd/fetchctl"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/profile"
	"github.com/canonical/fetch-service/seclog"
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

	// Specify the path to the security log file
	SecLog string `long:"security-log-file" description:"Log security events to this file"`
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

	if opts.SecLog != "" {
		if err := seclog.Init(opts.SecLog); err != nil {
			printf("error: %v", err)
			return 1
		}
		defer seclog.Close()
	}

	// Start continuous profiling server
	pp := profile.NewProfiler(opts.ProfilePort)
	if opts.Profile {
		pp.Start()
	}

	svcOpts := getServiceOptions(opts)

	seclog.SysStart(&seclog.EventData{}, policyMode(opts))

	svc, err := service.New(svcOpts)
	if err != nil {
		logger.Fatalf("Cannot create service: %s", err)
	}

	if err := svc.Start(); err != nil {
		logger.Fatalf("Cannot start service: %s", err)
	}

	// Shut down gracefully if terminated.
	cs := make(chan os.Signal, 1)
	signal.Notify(cs, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1)

	status := 0
	reason := ""

loop:
	for {
		select {
		case sig := <-cs:
			if sig == syscall.SIGUSR1 {
				logger.Reopen()
				seclog.Reopen()
				continue
			}
			logger.Infof("Exiting on %s signal.\n", sig)
			if sysSig, ok := sig.(syscall.Signal); ok {
				status = 128 + int(sysSig)
				reason = fmt.Sprintf("%s received", unix.SignalName(sysSig))
			} else {
				status = 128
				reason = sig.String()
			}
			break loop

		case <-svc.Dying():
			if err := svc.Err(); err != nil {
				reason = fmt.Sprintf("Server error: %s", err)
				logger.Error(reason)
				status = 2
			} else {
				// something called Stop()
				reason = "Server stopped"
				logger.Info(reason)
			}
			break loop

		case <-pp.Dying():
			// profiling server died
			reason = fmt.Sprintf("Profiling error: %s", pp.Err())
			logger.Error(reason)
			status = 3
		}
	}

	if err := svc.Stop(); err != nil {
		if status != 0 {
			// The primary failure reason (service/profiler/signal) was already
			// captured in status/reason. Don't override it with shutdown noise.
			logger.Errorf("error while stopping service: %s", err)
		} else {
			logger.Errorf("error: %s", err)
			status = 1
		}
	}

	seclog.SysShutdown(&seclog.EventData{Reason: reason})

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

func policyMode(opts CmdlineOptions) string {
	if opts.PermissiveMode {
		return "permissive-allowed"
	}
	return "strict-only"
}

func main() {
	cmd := filepath.Base(os.Args[0])
	if cmd == "fetchctl" || cmd == "fetch-service.fetchctl" {
		os.Exit(fetchctl.Run())
	}

	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			var description string
			var event string

			switch err := r.(type) {
			case runtime.Error:
				description = err.Error()

				msg := err.Error()
				if strings.Contains(msg, "nil pointer dereference") {
					event = "nil_pointer_dereference"
				} else if strings.Contains(msg, "out of range") {
					event = "index_out_of_bounds"
				} else {
					event = "runtime_error"
				}
			case error:
				description = err.Error()
				event = "panic"
			default:
				description = fmt.Sprintf("%v", r)
				event = "other"
			}

			seclog.SysCrash(&seclog.EventData{}, event, description, stack)

			fmt.Fprintf(os.Stderr, "panic: %v\n\n", r)
			os.Stderr.Write(debug.Stack())
			os.Exit(2)
		}
	}()

	os.Exit(Run())
}
