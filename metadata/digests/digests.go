// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2024 Canonical Ltd.
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

package digests

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
)

// Sha1Digest contains a 120-bit SHA1 digest.
type Sha1Digest [20]byte

func NewSha1Digest(digest string) (Sha1Digest, error) {
	h, err := hex.DecodeString(digest)
	if err != nil {
		return Sha1Digest{}, err
	}
	if len(h) != 20 { // SHA1 digest length is 160 bits
		return Sha1Digest{}, fmt.Errorf("SHA1 digest length (%d) is invalid", len(h))
	}
	return *(*Sha1Digest)(h), nil
}

// String obtains the SHA1 digest value as a hexadecimal string.
func (h Sha1Digest) String() string {
	return hex.EncodeToString(h[:])
}

// MarshalJSON marshals a SHA1 digest as a hexadecimal string.
func (h Sha1Digest) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(h.String())), nil
}

// UnmarshalJSON unmarshals a hexadecimal string representation of
// a SHA1 digest back to binary format.
func (h *Sha1Digest) UnmarshalJSON(data []byte) (err error) {
	d, err := strconv.Unquote(string(data))
	if err != nil {
		return
	}

	if len(d) != 40 {
		return errors.New("invalid SHA1 digest")
	}

	v, err := hex.DecodeString(d)
	if err != nil {
		return
	}

	copy((*h)[:], v)
	return
}

// Sha256Digest contains a 256-bit SHA1 digest
type Sha256Digest [32]byte

func NewSha256Digest(digest string) (Sha256Digest, error) {
	h, err := hex.DecodeString(digest)
	if err != nil {
		return Sha256Digest{}, err
	}
	if len(h) != 32 { // SHA256 digest length is 256 bits
		return Sha256Digest{}, fmt.Errorf("SHA256 digest length (%d) is invalid", len(h))
	}
	return *(*Sha256Digest)(h), nil
}

// String obtains the SHA256 digest value as a hexadecimal string.
func (h Sha256Digest) String() string {
	return hex.EncodeToString(h[:])
}

// MarshalJSON marshals a SHA256 digest as a hexadecimal string.
func (h Sha256Digest) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(h.String())), nil
}

// UnmarshalJSON unmarshals a hexadecimal string representation of
// a SHA256 digest back to binary format.
func (h *Sha256Digest) UnmarshalJSON(data []byte) (err error) {
	d, err := strconv.Unquote(string(data))
	if err != nil {
		return
	}

	if len(d) != 64 {
		return errors.New("invalid SHA256 digest")
	}

	v, err := hex.DecodeString(d)
	if err != nil {
		return
	}

	copy((*h)[:], v)
	return
}
