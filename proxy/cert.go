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

package proxy

import (
	"crypto/tls"
	"crypto/x509"
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
