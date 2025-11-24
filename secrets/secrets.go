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

	"github.com/canonical/fetch-service/glob"
)

type SecretType string

type Secret struct {
	Type       SecretType
	URL        glob.Glob
	BasicCreds string `json:"basic-credentials"`
}

// BasicAuthType is the only currently supported secret type
const BasicAuthType SecretType = "basic-auth"

// Error constants
var (
	ErrMissingSecretType = errors.New("Invalid secret: missing type")
	ErrInvalidSecretType = errors.New("Invalid secret: invalid type")
	ErrMissingSecretURL  = errors.New("Invalid secret: missing url")
	ErrMissingBasicCreds = errors.New("Invalid secret: missing credentials for 'basic-auth'")
)

func ValidateSecrets(sec []Secret) error {
	for _, s := range sec {
		if s.Type == "" {
			return ErrMissingSecretType
		}
		if s.Type != BasicAuthType {
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
	if sec.Type == BasicAuthType {
		if sec.BasicCreds == "" {
			return ErrMissingBasicCreds
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
	if s.Type == BasicAuthType {
		cred := base64.StdEncoding.EncodeToString([]byte(s.BasicCreds))
		req.Header.Set("Authorization", "Basic "+cred)
	}
}
