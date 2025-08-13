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
	"io"

	"github.com/canonical/fetch-service/logger"
)

var (
	NewAssertion      = newAssertion
	DecodeSignature   = decodeSignature
	ComputeDigestImpl = computeDigestImpl
	EncodeDigestImpl  = encodeDigestImpl

	DownloadSnapRevisionAssertionImpl    = downloadSnapRevisionAssertion
	DownloadSnapDeclarationAssertionImpl = downloadSnapDeclarationAssertion
	DownloadAccountAssertionImpl         = downloadAccountAssertion

	CheckSnapDeclarationFilter = checkSnapDeclarationFilter
)

type Assertion = assertion

func MockComputeDigest(mock func(io.Reader) ([]byte, error)) (restorer func()) {
	old := computeDigest
	computeDigest = mock
	return func() {
		computeDigest = old
	}
}

func MockEncodeDigest(mock func([]byte) (string, error)) (restorer func()) {
	old := encodeDigest
	encodeDigest = mock
	return func() {
		encodeDigest = old
	}
}

func MockDownloadSnapRevisionAssertion(mock func(string, logger.Logger) (*assertion, error)) (restorer func()) {
	old := downloadSnapRevisionAssertion
	downloadSnapRevisionAssertion = mock
	return func() {
		downloadSnapRevisionAssertion = old
	}
}

func MockDownloadSnapDeclarationAssertion(mock func(string, logger.Logger) (*assertion, error)) (restorer func()) {
	old := downloadSnapDeclarationAssertion
	downloadSnapDeclarationAssertion = mock
	return func() {
		downloadSnapDeclarationAssertion = old
	}
}

func MockDownloadAccountAssertion(mock func(string, logger.Logger) (*assertion, error)) (restorer func()) {
	old := downloadAccountAssertion
	downloadAccountAssertion = mock
	return func() {
		downloadAccountAssertion = old
	}
}
