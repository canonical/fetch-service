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

package apt_test

import (
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-mmap/mmap"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/apt"
	"github.com/canonical/fetch-service/metadata"
)

const (
	releaseURL = "http://archive.ubuntu.com/ubuntu/dists/jammy/InRelease"
)

// XXX: This file contains minimal testing for apt file formats. Tests
//      will be extended after the metadata format is approved.

func (s *aptSuite) TestAptReleaseInspector(c *C) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	req, err := http.NewRequest("GET", releaseURL, nil)
	c.Assert(err, IsNil)

	resp, err := client.Do(req)
	c.Assert(err, IsNil)
	c.Assert(resp.StatusCode, Equals, 200)

	defer resp.Body.Close()

	tmp := c.MkDir()
	dest, err := os.Create(filepath.Join(tmp, "290d07339dde2735121ab03e525ca6593c395a42.bin"))
	c.Assert(err, IsNil)

	_, err = io.Copy(dest, resp.Body)
	c.Assert(err, IsNil)

	dest.Close()

	h, _ := metadata.NewSha1Digest("290d07339dde2735121ab03e525ca6593c395a42")
	a := metadata.NewArtefact()
	a.Metadata.Type = "application/x-apt-release"
	a.Metadata.Sha1 = h

	//var iface inspectors.Inspector
	ins := apt.AptReleaseInspector{}
	//c.Assert(ins, Implements, &iface)

	f, err := mmap.Open(filepath.Join(tmp, "290d07339dde2735121ab03e525ca6593c395a42.bin"))
	c.Assert(err, IsNil)
	defer f.Close()

	stop, err := ins.InspectArtefact(f, a)
	c.Assert(err, IsNil)
	c.Assert(stop, Equals, true)

	c.Check(a.Metadata.Name, Equals, "InRelease")
	c.Check(a.Metadata.Vendor, Equals, "Ubuntu")
	c.Check(a.Metadata.Description, Equals, "Ubuntu Jammy 22.04")
	c.Check(a.Metadata.Author, Equals, "Ubuntu")
	//c.Check(a.Metadata.Annotations, HasLen, 0)
}
