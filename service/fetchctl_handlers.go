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
	"encoding/json"
	"fmt"
	"time"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
	"github.com/canonical/fetch-service/utils"
	"github.com/canonical/fetch-service/version"
	"gopkg.in/yaml.v3"
)

func handleFetchCtl(v messages.FetchCtl, svc *Service) messages.FetchCtlResult {
	switch v.Operation {
	case "version":
		return messages.FetchCtlResult{
			Status:  "ok",
			Message: version.Version,
		}
	case "update-config":
		return fetchCtlUpdateConfig(v, svc)

	case "update-cert":
		return fetchCtlUpdateCert(v, svc.opt.CertPath, svc.opt.KeyPath)

	case "create-session":
		return fetchCtlCreateSession(v, svc)

	case "list-artifacts":
		return fetchCtlListArtifacts(v)

	default:
		return messages.FetchCtlResult{
			Status:  "error",
			Message: "unsupported operation",
		}
	}
}

func fetchCtlUpdateConfig(v messages.FetchCtl, svc *Service) messages.FetchCtlResult {
	err := configUpdateConfig(v.Type, v.ValidateOnly, v.Payload, svc.opt.Config)
	if err != nil {
		logger.Warningf("service: %s update error: %s", v.Type, err.Error())
		return messages.FetchCtlResult{
			Status:  "error",
			Message: fmt.Sprintf("%s configuration update error", v.Type),
		}
	} else if v.ValidateOnly {
		logger.Infof("service: %s configuration validated", v.Type)
		return messages.FetchCtlResult{
			Status:  "ok",
			Message: "configuration validated",
		}
	}

	logger.Infof("service: %s configuration updated", v.Type)
	return messages.FetchCtlResult{
		Status:  "ok",
		Message: "configuration updated",
	}

}

func fetchCtlUpdateCert(v messages.FetchCtl, certPath, keyPath string) messages.FetchCtlResult {
	err := proxyUpdateCert(v.ValidateOnly, v.Payload, certPath, keyPath)
	if err != nil {
		logger.Warningf("service: certificate update error: %s", err.Error())
		return messages.FetchCtlResult{
			Status:  "error",
			Message: "certificate update error",
		}
	} else if v.ValidateOnly {
		logger.Info("service: proxy certificate updated")
		return messages.FetchCtlResult{
			Status:  "ok",
			Message: "certificate validated",
		}
	}

	logger.Info("service: certificate updated")
	return messages.FetchCtlResult{
		Status:  "ok",
		Message: "proxy certificate updated",
	}
}

func fetchCtlCreateSession(v messages.FetchCtl, svc *Service) messages.FetchCtlResult {
	var params messages.CreateSessionPayload
	err := json.Unmarshal([]byte(v.Payload), &params)
	if err != nil {
		return messages.FetchCtlResult{Status: "error", Message: fmt.Sprintf("malformed payload: %s", err)}
	}

	permissive := svc.opt.PermissiveMode && params.Mode == "permissive"
	timeout := time.Duration(params.Timeout) * time.Second

	cfg := config.SessionInspectorsConfig{}
	if len(params.InspectorsConfig) > 0 {
		err = yaml.Unmarshal(params.InspectorsConfig, &cfg)
		if err != nil {
			return messages.FetchCtlResult{Status: "error", Message: fmt.Sprintf("malformed payload: %s", err)}
		}
	}

	s := sessionNewWithId(params.SessionId, params.Token, svc.opt.Spool, timeout, permissive, nil, cfg)

	logger.Infof("service: session %s created", s.Id)
	svc.totalSessions++
	return messages.FetchCtlResult{
		Status:  "ok",
		Message: fmt.Sprintf("session %s:%s created (%s)", s.Id, s.Token, s.Metadata().Policy),
	}
}

func fetchCtlListArtifacts(v messages.FetchCtl) messages.FetchCtlResult {
	sessionId := string(v.Payload)
	s := session.GetSession(sessionId)
	if s == nil {
		return messages.FetchCtlResult{Status: "error", Message: "session does not exist"}
	}

	j, err := utils.JSONMarshalIndentNoHTMLEscape(s.Artifacts(), "", "   ")
	if err != nil {
		return messages.FetchCtlResult{Status: "error", Message: err.Error()}
	}

	return messages.FetchCtlResult{
		Status:  "ok",
		Message: string(j),
	}
}
