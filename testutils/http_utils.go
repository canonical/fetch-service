// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023 Canonical Ltd.
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

package testutils

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func HTTPDownload(url, filename string) error {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status code %d", resp.StatusCode)
	}

	defer func() { _ = resp.Body.Close() }()

	dest, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = dest.Close() }()

	_, err = io.Copy(dest, resp.Body)

	return err
}
