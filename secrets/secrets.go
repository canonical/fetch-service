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

	// ErrKeystoneV3BodyTooLarge is returned by InjectSecrets when a
	// keystone-v3 auth request body exceeds maxKeystoneV3BodySize.
	ErrKeystoneV3BodyTooLarge = errors.New("keystone-v3 request body exceeds maximum size")
)

// maxKeystoneV3BodySize bounds how much of a keystone-v3 request body
// injectSecret will buffer in memory. Real Keystone v3 auth-token
// requests (identity plus scope) are a few KB at most; this is
// generous headroom while preventing a large or chunked
// client-controlled body from being fully buffered in memory.
const maxKeystoneV3BodySize = 1 << 20 // 1 MiB

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

func InjectSecrets(secrets []Secret, url string, req *http.Request, sl logger.Logger) (bool, error) {
	for _, s := range secrets {
		if s.URL.Match(url) {
			if err := injectSecret(s, req, sl); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

func injectSecret(s Secret, req *http.Request, sl logger.Logger) error {
	switch s.Type {
	case BasicAuthType:
		cred := base64.StdEncoding.EncodeToString([]byte(s.BasicCreds))
		req.Header.Set("Authorization", "Basic "+cred)
	case MacaroonType:
		// Note that the macaroon is already base64-encoded, since it's possibly an
		// arbitrary sequence of bytes
		req.Header.Set("Authorization", "macaroon "+s.MacaroonCreds)
	case KeystoneV3Type:
		newBody, err := injectKeystoneV3Secret(s, req)
		if err != nil {
			// Forwarding raw here would silently send the caller's own,
			// unsubstituted credentials upstream instead of the injected
			// secret - exactly the untraceable failure this whole
			// feature exists to fix. Reject the request instead.
			return err
		}
		if newBody == nil {
			// Nothing to inject into (e.g. a bodyless GET matched the
			// secret's URL rule); leave the request untouched.
			return nil
		}

		req.Body = io.NopCloser(bytes.NewReader(newBody))
		req.ContentLength = int64(len(newBody))
		req.Header.Set("Content-Length", strconv.Itoa(len(newBody)))
		req.TransferEncoding = nil
	}
	return nil
}

type Identity struct {
	Methods               []string               `json:"methods"`
	Password              *Password              `json:"password,omitempty"`
	ApplicationCredential *ApplicationCredential `json:"application_credential,omitempty"`
}

type Password struct {
	User *User `json:"user"`
}

type User struct {
	Name     string         `json:"name"`
	Password string         `json:"password"`
	Domain   map[string]any `json:"domain"`
}

// ApplicationCredential is the identity.application_credential object
// of a Keystone v3 application-credential auth request. Unlike
// password auth it carries no user/domain scope: the credential is
// already bound to a single project when it is created.
type ApplicationCredential struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
}

// Keystone V3 request formats:
//
// Password auth:
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
//
// Application-credential auth:
//
// {
//   "auth": {
//     "identity": {
//       "methods": [
//         "application_credential"
//       ],
//       "application_credential": {
//         "id": "...",
//         "secret": "..."
//       }
//     }
//   }
// }

func injectKeystoneV3Secret(s Secret, req *http.Request) ([]byte, error) {
	if req.Body == nil {
		// Nothing to inject into (e.g. a bodyless GET matched the
		// secret's URL rule); leave the request untouched rather
		// than dereferencing a nil body.
		return nil, nil
	}

	raw, err := io.ReadAll(io.LimitReader(req.Body, maxKeystoneV3BodySize+1))
	req.Body.Close()
	if err != nil {
		// Only a prefix of the body was read. There is no complete
		// request left to forward, original or rewritten, so the
		// caller must reject this request instead of us silently
		// forwarding truncated JSON as if it were whole.
		return nil, fmt.Errorf("cannot read keystone-v3 request body: %w", err)
	}
	if len(raw) > maxKeystoneV3BodySize {
		return nil, fmt.Errorf("%w: got at least %d bytes", ErrKeystoneV3BodyTooLarge, len(raw))
	}

	id, secret, ok := strings.Cut(s.KeystoneV3Creds, ":")
	if !ok {
		return nil, errors.New("invalid keystone-v3 credentials format")
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("cannot decode keystone-v3 request body: %w", err)
	}

	authData, ok := body["auth"]
	if !ok {
		return nil, errors.New("no auth field in keystone-v3 request body")
	}

	var auth map[string]json.RawMessage
	if err := json.Unmarshal(authData, &auth); err != nil {
		return nil, fmt.Errorf("cannot unmarshal auth data: %w", err)
	}

	newIdentity, err := newKeystoneV3Identity(auth, id, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot build keystone-v3 identity: %w", err)
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

// newKeystoneV3Identity builds the replacement "identity" object for
// a Keystone v3 auth request, preserving whichever auth method the
// original request used and substituting id/secret for its
// credentials. Application-credential auth is already project-scoped
// by the credential itself, so it carries no domain; password auth
// keeps the original request's user domain, since Keystone needs it
// to resolve the user.
func newKeystoneV3Identity(auth map[string]json.RawMessage, id, secret string) (map[string]any, error) {
	identityData, ok := auth["identity"]
	if !ok {
		return nil, errors.New("cannot find identity in keystone-v3 auth request")
	}

	var identity Identity
	if err := json.Unmarshal(identityData, &identity); err != nil {
		return nil, fmt.Errorf("cannot unmarshal identity data: %w", err)
	}

	if len(identity.Methods) != 1 {
		return nil, fmt.Errorf("unsupported keystone-v3 auth methods %v: exactly one method is supported", identity.Methods)
	}

	switch identity.Methods[0] {
	case "application_credential":
		if identity.ApplicationCredential == nil {
			return nil, errors.New("keystone-v3 identity method is application_credential but application_credential object is missing")
		}
		return map[string]any{
			"methods": []string{"application_credential"},
			"application_credential": map[string]any{
				"id":     id,
				"secret": secret,
			},
		}, nil

	case "password":
		if identity.Password == nil || identity.Password.User == nil {
			return nil, errors.New("keystone-v3 identity method is 'password' but password.user object is missing")
		}

		domain, err := getKeystoneV3IdentityDomain(auth)
		if err != nil {
			return nil, fmt.Errorf("cannot read keystone-v3 identity domain: %w", err)
		}

		return map[string]any{
			"methods": []string{"password"},
			"password": map[string]any{
				"user": map[string]any{
					"name":     id,
					"password": secret,
					"domain":   domain,
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported keystone-v3 auth method: %q", identity.Methods[0])
	}
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
