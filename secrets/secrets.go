// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2025 Canonical Ltd.
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

package secrets

import (
	"encoding/base64"
	"errors"
	"net/http"
	"slices"

	"github.com/canonical/fetch-service/glob"
)

type SecretType string

type Secret struct {
	Type          SecretType
	URL           glob.Glob
	BasicCreds    string `json:"basic-credentials"`
	MacaroonCreds string `json:"macaroon-credentials"`
}

// Supported secret types

const BasicAuthType SecretType = "basic-auth"
const MacaroonType SecretType = "macaroon"

func getSecretTypes() []SecretType {
	return []SecretType{BasicAuthType, MacaroonType}
}

// Error constants
var (
	ErrMissingSecretType    = errors.New("Invalid secret: missing type")
	ErrInvalidSecretType    = errors.New("Invalid secret: invalid type")
	ErrMissingSecretURL     = errors.New("Invalid secret: missing url")
	ErrMissingBasicCreds    = errors.New("Invalid secret: missing credentials for 'basic-auth'")
	ErrMissingMacaroonCreds = errors.New("Invalid secret: missing credentials for 'macaroon'")
)

func ValidateSecrets(sec []Secret) error {
	for _, s := range sec {
		if s.Type == "" {
			return ErrMissingSecretType
		}
		if !slices.Contains(getSecretTypes(), s.Type) {
			return ErrInvalidSecretType
		}
		if s.URL.G == nil {
			return ErrMissingSecretURL
		}
		if err := validateCredentials(s); err != nil {
			return err
		}
	}
	return nil
}

func validateCredentials(sec Secret) error {
	switch sec.Type {
	case BasicAuthType:
		if sec.BasicCreds == "" {
			return ErrMissingBasicCreds
		}
	case MacaroonType:
		if sec.MacaroonCreds == "" {
			return ErrMissingMacaroonCreds
		}
	}
	return nil
}

func InjectSecrets(secrets []Secret, url string, req *http.Request) bool {
	for _, s := range secrets {
		if s.URL.Match(url) {
			injectSecret(s, req)
			return true
		}
	}
	return false
}

func injectSecret(s Secret, req *http.Request) {
	switch s.Type {
	case BasicAuthType:
		cred := base64.StdEncoding.EncodeToString([]byte(s.BasicCreds))
		req.Header.Set("Authorization", "Basic "+cred)
	case MacaroonType:
		// Note that the macaroon is already base64-encoded, since it's possibly an
		// arbitrary sequence of bytes
		req.Header.Set("Authorization", "macaroon "+s.MacaroonCreds)
	}
}
