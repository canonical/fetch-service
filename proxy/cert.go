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

package proxy

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"github.com/elazarl/goproxy"

	"github.com/canonical/fetch-service/logger"
)

func CreateProxyCA(cert, key []byte) (tls.Certificate, error) {
	logger.Info("create CA from PEM certificate and key blocks")

	ca, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	if ca.Leaf, err = x509.ParseCertificate(ca.Certificate[0]); err != nil {
		return tls.Certificate{}, err
	}

	return ca, nil
}

// SetProxyCA enables the HTTPS proxy MITM certificate
func SetProxyCA(ca tls.Certificate) error {
	goproxy.GoproxyCa = ca
	goproxy.OkConnect = &goproxy.ConnectAction{
		Action:    goproxy.ConnectAccept,
		TLSConfig: goproxy.TLSConfigFromCA(&ca),
	}
	goproxy.MitmConnect = &goproxy.ConnectAction{
		Action:    goproxy.ConnectMitm,
		TLSConfig: goproxy.TLSConfigFromCA(&ca),
	}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{
		Action:    goproxy.ConnectHTTPMitm,
		TLSConfig: goproxy.TLSConfigFromCA(&ca),
	}
	goproxy.RejectConnect = &goproxy.ConnectAction{
		Action:    goproxy.ConnectReject,
		TLSConfig: goproxy.TLSConfigFromCA(&ca),
	}

	return nil
}

func UpdateCert(dryRun bool, payload []byte, certPath, keyPath string) error {
	cert, key, err := splitCertKey(payload)
	if err != nil {
		return err
	}

	// Validate certificate and key PEM block data
	ca, err := CreateProxyCA(cert, key)
	if err != nil {
		return err
	}

	if !dryRun {
		if err := SetProxyCA(ca); err != nil {
			return err
		}

		// Overwrite files only if the data is valid
		if err := UpdateCertFiles(certPath, keyPath, cert, key); err != nil {
			return err
		}
		logger.Info("cert: write certificate and key files")
	}

	return nil
}

// LoadCertificate loads the proxy MITM certificates from the file system.
func LoadCertificate(certPath, keyPath string) ([]byte, []byte, error) {
	if certPath == "" {
		return nil, nil, fmt.Errorf("HTTPS proxy certificate path not specified")
	}
	logger.Infof("Loading certificate from %s", certPath)
	cert, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}

	if keyPath == "" {
		return nil, nil, fmt.Errorf("HTTPS proxy key path not specified")
	}
	logger.Infof("Loading key from %s", keyPath)
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}

func UpdateCertFiles(certPath, keyPath string, cert, key []byte) error {
	tmpCertPath := certPath + ".new"
	tmpKeyPath := keyPath + ".new"

	// Create temporary files
	if err := os.WriteFile(tmpCertPath, cert, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(tmpKeyPath, key, 0644); err != nil {
		os.Remove(tmpCertPath)
		return err
	}

	// Rename cert and key files
	// Not fully atomic!
	if err := os.Rename(tmpCertPath, certPath); err != nil {
		return err
	}
	if err := os.Rename(tmpKeyPath, keyPath); err != nil {
		return fmt.Errorf("inconsistent state: %w", err)
	}

	return nil
}

func splitCertKey(content []byte) ([]byte, []byte, error) {
	s := bytes.SplitN(content, []byte("\n\n"), 2)
	if len(s) != 2 {
		return nil, nil, errors.New("cannot parse certificate and key")
	}

	return s[0], s[1], nil
}
