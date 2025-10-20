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
	"errors"

	"github.com/canonical/fetch-service/glob"
)

type Secret struct {
	Type string
	Url  glob.Glob
}

var (
	ErrMissingSecretType = errors.New("Invalid secret: missing type")
	ErrInvalidSecretType = errors.New("Invalid secret: invalid type")
	ErrMissingSecretUrl  = errors.New("Invalid secret: missing url")
)

func ValidateSecrets(sec []Secret) error {
	for _, s := range sec {
		if s.Type == "" {
			return ErrMissingSecretType
		}
		if s.Type != "basic-auth" {
			return ErrInvalidSecretType
		}
		if s.Url.G == nil {
			return ErrMissingSecretUrl
		}
	}
	return nil
}
