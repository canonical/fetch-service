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

package acl

import (
	"net"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/service/config"
)

// Allowed verifies if the request address can be accessed according
// to the global ACL rules.
func Allowed(ip net.IP) bool {
	if ip == nil {
		return false
	}

	logger.Debugf("acl: check if connection to %s is allowed", ip.String())
	cfg := config.GetHTTPProxyConfig()

	for _, rule := range cfg.Rules {
		allowed := rule.Access == config.Allow

		// for each rule, see if the address matches
		for _, dst := range rule.Dst {
			if dst.Contains(ip) {
				return allowed
			}
		}
	}

	// No rule matched, use global policy.
	return cfg.Policy == config.Allow
}
