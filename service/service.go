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
	p        *proxy.HttpProxy // proxy instance
	ctl      *control.Server  // control server
	fetchctl *fetchctl.Server // configuration server
	ch       chan interface{} // channel to get feedback from handlers
	start    time.Time        // service start time (UTC)
	opt      *Options         // configuration options
	tomb     tomb.Tomb        // service dispatcher loop reaper

	totalSessions uint64 // number of created sessions
}

var (
	proxyNewHttpProxy = proxy.NewHttpProxy
	controlNewServer  = control.NewServer
	fetchctlNewServer = fetchctl.NewServer
	sessionNewWithId  = session.NewWithId
)

func New(opt *Options) (*Service, error) {
	// obtain authentication credentials from the environment
	creds := os.Getenv("FETCH_SERVICE_AUTH")

	cert, key, err := proxy.LoadCertificate(opt.CertPath, opt.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("Cannot load certificates: %s", err)
	}

	ch := make(chan interface{})
	p, err := proxyNewHttpProxy(opt.ProxyPort, opt.Spool, cert, key, ch)
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

	if err := loadHttpProxyRulesOrDefault(svc.opt.Config); err != nil {
		return fmt.Errorf("cannot load proxy rules: %s", err)
	}

	if err := loadInspectorsConfigOrDefault(svc.opt.Config); err != nil {
		return fmt.Errorf("cannot load inspectors configuration: %s", err)
	}

	logger.Info("Starting service...")

	if err := svc.p.Start(); err != nil {
		return err
	}

	if err := svc.fetchctl.Start(); err != nil {
		return err
	}

	svc.ctl.Start()

	svc.tomb.Go(svc.dispatcher)

	return nil
}

var (
	configLoadHttpProxyRules   = config.LoadHttpProxyRules
	configLoadInspectorsConfig = config.LoadInspectorsConfig
)

func loadHttpProxyRulesOrDefault(cfgdir string) error {
	err := configLoadHttpProxyRules(cfgdir)
	if errors.Is(err, os.ErrNotExist) {
		if RunningFromSnap() {
			err = configLoadHttpProxyRules(os.ExpandEnv(SnapBundledConfigDir))
		} else {
			logger.Infof("ACL configuration file does not exist in %s", cfgdir)
			return nil
		}
	}
	return err
}

func loadInspectorsConfigOrDefault(cfgdir string) error {
	err := configLoadInspectorsConfig(cfgdir)
	if errors.Is(err, os.ErrNotExist) {
		if RunningFromSnap() {
			err = configLoadInspectorsConfig(os.ExpandEnv(SnapBundledConfigDir))
		} else {
			logger.Infof("Inspectors configuration file does not exist in %s", cfgdir)
			return nil
		}
	}
	return err
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

loop:
	for {
		select {
		case msg := <-svc.ch:
			if svc.opt.IdleShutdown > 0 {
				idleTimer.Reset(time.Duration(svc.opt.IdleShutdown) * time.Second)
			}
			logger.Infof("[service] received message: %T", msg)

			handleMessages(svc, msg)

		case sessionId := <-session.ExpiredSessionId:
			logger.Infof("[%s] session expired", sessionId)
			s := session.GetSession(sessionId)
			if s == nil {
				logger.Warningf("[service] session %s does not exist", sessionId)
				break
			}
			if err := s.Finish(); err != nil {
				logger.Errorf("[%s] cannot finish session: %s", sessionId, err)
			}
			if err := session.RemoveResources(svc.opt.Spool, sessionId); err != nil {
				logger.Errorf("[%s] cannot remove session resources: %s", sessionId, err)
			}
			if svc.opt.IdleShutdown > 0 {
				idleTimer.Reset(time.Duration(svc.opt.IdleShutdown) * time.Second)
			}

		case <-svc.tomb.Dying():
			return svc.tomb.Err()

		case <-svc.ctl.Dying():
			return svc.ctl.Err()

		case <-svc.fetchctl.Dying():
			return svc.fetchctl.Err()

		case <-svc.p.Dying():
			return svc.p.Err()

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
	session.FinishAll()

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
	return nil
}

func (svc *Service) Alive() bool {
	return svc.tomb.Alive()
}

func (svc *Service) Dying() <-chan struct{} {
	return svc.tomb.Dying()
}
