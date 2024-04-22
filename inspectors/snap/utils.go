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

package snap

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

const (
	hashDigestBufSize = 2 * 1024 * 1024
)

var (
	v1FixedTimestamp = time.Date(2016, time.January, 1, 0, 0, 0, 0, time.UTC)
)

// fileDigest computes a sha3-384 digest of the given file. It also
// returns the file size.
func computeDigest(f io.Reader) ([]byte, error) {
	h := crypto.SHA3_384.New()
	_, err := io.CopyBuffer(h, f, make([]byte, hashDigestBufSize))
	if err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// encodeDigest encodes the digest from hash algorithm to be put in
// an assertion header.
func encodeDigest(digest []byte) (string, error) {
	size := crypto.SHA3_384.Size()
	if len(digest) != size {
		return "", fmt.Errorf("hash digest must have %d bytes", size)
	}
	return base64.RawURLEncoding.EncodeToString(digest), nil
}

// decodeSignature deserializes a signature.
func decodeSignature(signature []byte) (*packet.Signature, error) {
	pkt, err := decodeV1(signature, "signature")
	if err != nil {
		return nil, err
	}
	sig, ok := pkt.(*packet.Signature)
	if !ok {
		return nil, fmt.Errorf("expected signature, got instead: %T", pkt)
	}
	return sig, nil
}

// decodePublicKey deserializes a public key.
func decodePublicKey(pubKey []byte) (*packet.PublicKey, error) {
	pkt, err := decodeV1(pubKey, "public key")
	if err != nil {
		return nil, err
	}
	pubk, ok := pkt.(*packet.PublicKey)
	if !ok {
		return nil, fmt.Errorf("expected public key, got instead: %T", pkt)
	}
	rsaPubKey, ok := pubk.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("expected RSA public key, got instead: %T", pubk.PublicKey)
	}
	return packet.NewRSAPublicKey(v1FixedTimestamp, rsaPubKey), nil
}

// OpenPGP packet decoder V1 as defined in snapd. See snapd/asserts/crypto.go
// for further details.
func decodeV1(b []byte, kind string) (packet.Packet, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("cannot decode %s: no data", kind)
	}
	buf := make([]byte, base64.StdEncoding.DecodedLen(len(b)))
	n, err := base64.StdEncoding.Decode(buf, b)
	if err != nil {
		return nil, fmt.Errorf("cannot decode %s: %v", kind, err)
	}
	if n == 0 {
		return nil, fmt.Errorf("cannot decode %s: base64 without data", kind)
	}
	buf = buf[:n]
	if buf[0] != 0x01 { // v1
		return nil, fmt.Errorf("unsupported %s format version: %d", kind, buf[0])
	}
	rd := bytes.NewReader(buf[1:])
	pkt, err := packet.Read(rd)
	if err != nil {
		return nil, fmt.Errorf("cannot decode %s: %v", kind, err)
	}
	if rd.Len() != 0 {
		return nil, fmt.Errorf("%s has spurious trailing data", kind)
	}
	return pkt, nil
}
