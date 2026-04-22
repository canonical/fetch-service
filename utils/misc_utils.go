// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2026 Canonical Ltd.
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

package utils

import (
	"fmt"
	"net"
	"net/http"
	"os"
)

// ServerIP returns the server IP address from request r.
func ServerIP(r *http.Request) string {
	localAddr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return "unknown"
	}

	ip, _, err := net.SplitHostPort(localAddr.String())
	if err != nil {
		return localAddr.String()
	}
	return ip
}

// ClientIP extracts the IP address from request r.
func ClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return fmt.Sprintf("%s (%s)", err, r.RemoteAddr)
	}
	return ip
}

// RuntimeEnv returns the runtime environment set using APP_ENV.
func RuntimeEnv() string {
	env := os.Getenv("APP_ENV")
	if env == "" {
		return "unknown"
	}
	return env
}
