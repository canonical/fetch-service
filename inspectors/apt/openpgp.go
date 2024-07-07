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

package apt

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	openpgp "github.com/ProtonMail/go-crypto/openpgp"
	armor "github.com/ProtonMail/go-crypto/openpgp/armor"
	packet "github.com/ProtonMail/go-crypto/openpgp/packet"

	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/utils"
)

var checkSignature = checkSignatureImpl

func checkSignatureImpl(f io.ReadSeeker, notes Annotation) (io.ReadSeeker, error) {
	public_key := os.Getenv("FETCH_APT_RELEASE_PUBLIC_KEY")
	if public_key == "" {
		return nil, fmt.Errorf("public key not found")
	}

	var keys []*packet.PublicKey
	var keyIds []string

	keyBlocks := strings.SplitAfter(public_key, "-----END PGP PUBLIC KEY BLOCK-----")
	for _, k := range keyBlocks {
		if strings.TrimSpace(k) == "" {
			continue
		}
		keyReader := strings.NewReader(k)
		key, err := decodePublicKey(keyReader)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
		keyIds = append(keyIds, key.KeyIdString())
	}

	notes.Add("public-keys", keyIds)

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	// From chisel internal/archive/archive.go:
	// Decode the signature(s) and verify the InRelease file. The InRelease
	// file may have multiple signatures from different keys. Verify that at
	// least one signature is valid against the archive's set of public keys.
	// Unlike gpg --verify which ensures the verification of all signatures,
	// this is in line with what apt does internally:
	// https://salsa.debian.org/apt-team/apt/-/blob/4e344a4/methods/gpgv.cc#L553-557
	sigs, body, err := utils.DecodeClearSigned(data)
	if err != nil {
		return nil, fmt.Errorf("cannot decode clearsigned file: %s", err)
	}

	logger.Debugf("number of signatures: %d", len(sigs))
	logger.Debugf("number of public keys: %d", len(keys))

	err = utils.VerifyAnySignature(keys, sigs, body)
	if err != nil {
		return bytes.NewReader(body), fmt.Errorf("cannot verify signature of the InRelease file")
	}

	return bytes.NewReader(body), err
}

func decodePublicKey(f io.Reader) (*packet.PublicKey, error) {
	block, err := armor.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("cannot decode key: %s", err)
	}

	if block.Type != openpgp.PublicKeyType {
		return nil, fmt.Errorf("invalid data type")
	}

	reader := packet.NewReader(block.Body)
	pkt, err := reader.Next()
	if err != nil {
		return nil, fmt.Errorf("error reading key: %s", err)
	}

	key, ok := pkt.(*packet.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error parsing key")
	}

	return key, nil
}
