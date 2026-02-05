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
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/logger"
)

type SecretType string

type Secret struct {
	Type            SecretType
	URL             glob.Glob
	BasicCreds      string `json:"basic-credentials"`
	MacaroonCreds   string `json:"macaroon-credentials"`
	KeystoneV3Creds string `json:"keystone-v3-credentials"`
}

// Supported secret types

const BasicAuthType SecretType = "basic-auth"
const MacaroonType SecretType = "macaroon"
const KeystoneV3Type SecretType = "keystone-v3"

func getSecretTypes() []SecretType {
	return []SecretType{BasicAuthType, MacaroonType, KeystoneV3Type}
}

// Error constants
var (
	ErrMissingSecretType      = errors.New("Invalid secret: missing type")
	ErrInvalidSecretType      = errors.New("Invalid secret: invalid type")
	ErrMissingSecretURL       = errors.New("Invalid secret: missing url")
	ErrMissingBasicCreds      = errors.New("Invalid secret: missing credentials for 'basic-auth'")
	ErrMissingMacaroonCreds   = errors.New("Invalid secret: missing credentials for 'macaroon'")
	ErrMissingKeystoneV3Creds = errors.New("Invalid secret: missing credentials for 'keystone-v3'")
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
	case KeystoneV3Type:
		if sec.KeystoneV3Creds == "" {
			return ErrMissingKeystoneV3Creds
		}
	}
	return nil
}

func InjectSecrets(secrets []Secret, url string, req *http.Request, slog logger.Logger) bool {
	for _, s := range secrets {
		if s.URL.Match(url) {
			injectSecret(s, req, slog)
			return true
		}
	}
	return false
}

func injectSecret(s Secret, req *http.Request, slog logger.Logger) {
	switch s.Type {
	case BasicAuthType:
		cred := base64.StdEncoding.EncodeToString([]byte(s.BasicCreds))
		req.Header.Set("Authorization", "Basic "+cred)
	case MacaroonType:
		// Note that the macaroon is already base64-encoded, since it's possibly an
		// arbitrary sequence of bytes
		req.Header.Set("Authorization", "macaroon "+s.MacaroonCreds)
	case KeystoneV3Type:
		newBody, err := injectKeystoneV3Secret(s, req.Body)
		if err != nil {
			slog.Debugf("cannot inject keystone-v3 secret: %s", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(newBody))
		req.ContentLength = int64(len(newBody))
		req.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
		req.TransferEncoding = nil
	}
}

type Identity struct {
	Methods  []string  `json:"methods"`
	Password *Password `json:"password,omitempty"`
}

type Password struct {
	User *User `json:"user"`
}

type User struct {
	Name     string         `json:"name"`
	Password string         `json:"password"`
	Domain   map[string]any `json:"domain"`
}

// Keystone V3 request format:
//
// {
//   "auth": {
//     "identity": {
//       "methods": [
//         "password"
//       ],
//       "password": {
//         "user": {
//           "domain": {
//             "name": "default"
//           },
//           "name": "...",
//           "password": "..."
//         }
//       }
//     },
//     "scope": {
//       "project": {
//         "domain": {
//           "name": "default"
//         },
//         "name": "..."
//       }
//     }
//   }
// }

func injectKeystoneV3Secret(s Secret, r io.ReadCloser) ([]byte, error) {
	user, pass, ok := strings.Cut(s.KeystoneV3Creds, ":")
	if !ok {
		return nil, errors.New("invalid keystone-v3 credentials format")
	}

	var body map[string]json.RawMessage
	dec := json.NewDecoder(r)
	err := dec.Decode(&body)
	if err != nil {
		return nil, fmt.Errorf("cannot decode keystone-v3 request body: %w", err)
	}
	r.Close()

	authData, ok := body["auth"]
	if !ok {
		return nil, fmt.Errorf("no auth field in keystone-v3 request body: %w", err)
	}

	var auth map[string]json.RawMessage
	err = json.Unmarshal(authData, &auth)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal auth data: %w", err)
	}

	domain, err := getKeystoneV3IdentityDomain(auth)
	if err != nil {
		return nil, fmt.Errorf("cannot read keystone-v3 identity domain: %w", err)
	}

	newIdentity := map[string]any{
		"methods": []string{"password"},
		"password": map[string]any{
			"user": map[string]any{
				"name":     user,
				"password": pass,
				"domain":   domain,
			},
		},
	}

	identityBytes, err := json.Marshal(newIdentity)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal keystone-v3 identity: %w", err)
	}
	auth["identity"] = identityBytes

	authBytes, err := json.Marshal(auth)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal keystone-v3 auth: %w", err)
	}
	body["auth"] = authBytes

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal keystone-v3 request body: %w", err)
	}

	return bodyBytes, nil
}

func getKeystoneV3IdentityDomain(auth map[string]json.RawMessage) (map[string]any, error) {
	identityData, ok := auth["identity"]
	if !ok {
		return nil, errors.New("cannot find identity in keystone-v3 auth request")
	}

	var identity Identity
	err := json.Unmarshal(identityData, &identity)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal identity data: %w", err)
	}

	if identity.Password == nil {
		return nil, errors.New("cannot find password in keystone-v3 auth request")
	}

	if identity.Password.User == nil {
		return nil, errors.New("cannot find user in keystone-v3 auth request")
	}

	return identity.Password.User.Domain, nil
}
