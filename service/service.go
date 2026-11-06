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

package service

import (
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"gopkg.in/tomb.v2"

	"github.com/canonical/fetch-service/control"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/service/fetchctl"
	"github.com/canonical/fetch-service/session"
)

const (
	SnapConfigDir        = "${SNAP_DATA}/conf"
	SnapSpoolDir         = "${SNAP_COMMON}/spool"
	NonSnapConfigDir     = "/etc/fetch"
	NonSnapSpoolDir      = "/var/lib/fetch"
	SnapBundledConfigDir = "${SNAP}/conf"
)

func RunningFromSnap() bool {
	return os.Getenv("SNAP") != "" && os.Getenv("SNAP_NAME") == "fetch-service"
}

// Service implements the fetch service main loop.
type Service struct {
	p        *proxy.HTTPProxy // proxy instance
	ctl      *control.Server  // control server
	fetchctl *fetchctl.Server // configuration server
	ch       chan interface{} // channel to get feedback from handlers
	start    time.Time        // service start time (UTC)
	opt      *Options         // configuration options
	tomb     tomb.Tomb        // service dispatcher loop reaper
	started  atomic.Bool      // true only after tomb.Go registered the dispatcher
	stopping atomic.Bool      // set when Stop() begins graceful shutdown

	totalSessions uint64 // number of created sessions
}

var (
	proxyNewHTTPProxy   = proxy.NewHTTPProxy
	controlNewServer    = control.NewServer
	fetchctlNewServer   = fetchctl.NewServer
	fetchctlServerStart = func(s *fetchctl.Server) error { return s.Start() }
	sessionNewWithID    = session.NewWithID
)

func New(opt *Options) (*Service, error) {
	// obtain authentication credentials from the environment
	creds := os.Getenv("FETCH_SERVICE_AUTH")

	cert, key, err := proxy.LoadCertificate(opt.CertPath, opt.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("Cannot load certificates: %s", err)
	}

	ch := make(chan interface{})
	p, err := proxyNewHTTPProxy(opt.ProxyPort, opt.Spool, cert, key, ch)
	if err != nil {
		return nil, err
	}

	return &Service{
		p:        p,
		ctl:      controlNewServer(opt.ControlPort, ch, creds),
		fetchctl: fetchctlNewServer(ch),
		opt:      opt,
		ch:       ch,
		start:    time.Now().UTC(),
	}, nil
}

// Start runs the fetch service dispatcher.
func (svc *Service) Start() error {
	logger.Info("Loading service configuration...")

	if err := loadHTTPProxyRulesOrDefault(svc.opt.Config); err != nil {
		return fmt.Errorf("cannot load proxy rules: %s", err)
	}

	if err := loadDefaultInspectorsConfigCombine(svc.opt.Config); err != nil {
		return fmt.Errorf("cannot load inspectors configuration: %s", err)
	}

	logger.Info("Starting service...")

	if err := svc.p.Start(); err != nil {
		return err
	}

	if err := fetchctlServerStart(svc.fetchctl); err != nil {
		return err
	}

	svc.ctl.Start()

	svc.tomb.Go(svc.dispatcher)
	svc.started.Store(true)

	return nil
}

var (
	configLoadHTTPProxyRules           = config.LoadHTTPProxyRules
	configLoadInspectorsConfig         = config.LoadInspectorsConfig
	configLoadOverrideInspectorsConfig = config.LoadOverrideInspectorsConfig
)

func loadHTTPProxyRulesOrDefault(cfgdir string) error {
	err := configLoadHTTPProxyRules(cfgdir)
	if errors.Is(err, os.ErrNotExist) {
		if RunningFromSnap() {
			err = configLoadHTTPProxyRules(os.ExpandEnv(SnapBundledConfigDir))
		} else {
			logger.Infof("ACL configuration file does not exist in %s", cfgdir)
			return nil
		}
	}
	return err
}

func loadDefaultInspectorsConfigCombine(cfgdir string) error {
	if !RunningFromSnap() {
		return configLoadInspectorsConfig(cfgdir)
	}
	err := configLoadInspectorsConfig(os.ExpandEnv(SnapBundledConfigDir))
	if err != nil {
		// Failing to load the configuration shipped in the snap is fatal error
		return err
	}
	err = configLoadOverrideInspectorsConfig(cfgdir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Infof("Inspectors configuration file does not exist in %s", cfgdir)
			return nil
		}
		return err
	}
	return nil
}

var (
	configUpdateConfig = config.UpdateConfig
	proxyUpdateCert    = proxy.UpdateCert
)

func (svc *Service) dispatcher() error {
	// Set up idle auto-shutdown
	idleTimer := time.NewTimer(time.Duration(svc.opt.IdleShutdown) * time.Second)
	if svc.opt.IdleShutdown == 0 {
		if !idleTimer.Stop() {
			<-idleTimer.C
		}
	}

	// Local copies of child Dying channels so we can nil them out when a
	// child dies cleanly during graceful shutdown (err == nil). This keeps
	// the dispatcher alive to process remaining in-flight messages until
	// svc.tomb itself is killed.
	ctlDying := svc.ctl.Dying()
	fetchctlDying := svc.fetchctl.Dying()
	proxyDying := svc.p.Dying()

loop:
	for {
		select {
		case msg := <-svc.ch:
			if svc.opt.IdleShutdown > 0 {
				idleTimer.Reset(time.Duration(svc.opt.IdleShutdown) * time.Second)
			}
			logger.Infof("service: received message: %T", msg)

			handleMessages(svc, msg)

		case sessionID := <-session.ExpiredSessionID:
			logger.Infof("service: session %s expired", sessionID)
			s := session.GetSession(sessionID)
			if s == nil {
				logger.Warningf("service: session %s does not exist", sessionID)
				break
			}
			if err := s.Finish(); err != nil {
				logger.Errorf("service: cannot finish session %s: %s", sessionID, err)
			}
			if err := session.RemoveResources(svc.opt.Spool, sessionID); err != nil {
				logger.Errorf("service: cannot remove session %s resources: %s", sessionID, err)
			}
			if svc.opt.IdleShutdown > 0 {
				idleTimer.Reset(time.Duration(svc.opt.IdleShutdown) * time.Second)
			}

		case <-svc.tomb.Dying():
			return svc.tomb.Err()

		// Child servers dying unexpectedly tears down the whole service.
		// During graceful shutdown (svc.stopping is set), deaths are
		// expected — ignore them so the dispatcher keeps processing
		// in-flight requests from other servers.
		case <-ctlDying:
			if !svc.stopping.Load() {
				return svc.ctl.Err()
			}
			ctlDying = nil

		case <-fetchctlDying:
			if !svc.stopping.Load() {
				return svc.fetchctl.Err()
			}
			fetchctlDying = nil

		case <-proxyDying:
			if !svc.stopping.Load() {
				return svc.p.Err()
			}
			proxyDying = nil

		case <-idleTimer.C:
			n := session.NumSessions()
			if n < 1 {
				logger.Infof("auto-shutdown after being idle for %d seconds", svc.opt.IdleShutdown)
				break loop
			} else {
				logger.Infof("number of active sessions: %d", n)
			}
		}
	}

	return nil
}

func (svc *Service) Stop() error {
	logger.Info("Stopping service...")

	// Signal the dispatcher that child deaths are expected from this point.
	// Without this, the dispatcher exits as soon as a child tomb dies during
	// graceful shutdown, leaving other children's in-flight requests blocked
	// on the dispatch channel with no reader.
	svc.stopping.Store(true)

	session.FinishAll()

	// Stop child servers first so their in-flight work can drain before the
	// main service loop is marked dead.
	if err := svc.p.Stop(); err != nil {
		return fmt.Errorf("cannot shut down the HTTP server: %w", err)
	}

	if err := svc.fetchctl.Stop(); err != nil {
		return fmt.Errorf("cannot shut down the fetchctl socket: %w", err)
	}

	if err := svc.ctl.Stop(); err != nil {
		return fmt.Errorf("cannot shut down the control API server: %w", err)
	}

	svc.tomb.Kill(nil)
	// Only wait for the dispatcher goroutine if Start() successfully
	// registered it. If Start() failed partway through (e.g. a listen
	// error), no goroutine was registered and Wait() would block forever.
	if svc.started.Load() {
		return svc.tomb.Wait()
	}
	return nil
}

func (svc *Service) Alive() bool {
	return svc.tomb.Alive()
}

func (svc *Service) Dying() <-chan struct{} {
	return svc.tomb.Dying()
}

func (svc *Service) Err() error {
	return svc.tomb.Err()
}
