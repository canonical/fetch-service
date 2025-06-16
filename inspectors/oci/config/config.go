// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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

package config

import (
	"github.com/canonical/fetch-service/glob"
)

// The OCI inspector has the following configuration format:
//
//	oci:
//	  registries:
//	    <registry-name>:
//	      url: <registry-url-pattern>
//	      auth-url: <registry-authentication-url-pattern>
//
// The top-level field "oci" is defined in ./service/config/config.go:275.
type OciInspectorConfig struct {
	Registries map[string]OciInspectorConfigRegistry `yaml:"registries"`
}

type OciInspectorConfigRegistry struct {
	// The registry URL e.g. https://registry-1.docker.io.
	Url glob.Glob `yaml:"url"`

	// The registry authentication URL e.g. https://auth.docker.io.
	// Ideally we would not have this and a request to <registry>/v2/ should
	// return a 401 response with "www-authenticate" header pointing to the
	// authentication URL. However, the fetch service inspectors do not get to
	// see the 401 response headers. See ./proxy/proxy.go:228.
	AuthUrl glob.Glob `yaml:"auth-url"`
}
