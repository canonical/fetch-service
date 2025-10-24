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

package service_test

import (
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service"
	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
)

func (t *serviceSuite) TestReuseInspectionResult(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.ch = ch
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	dir := c.MkDir()
	certPath, keyPath, err := createCertFiles(dir)
	c.Assert(err, IsNil)

	spoolDir := dir
	opt := service.Options{
		ProxyPort: 1337,
		Spool:     spoolDir,
		CertPath:  certPath,
		KeyPath:   keyPath,
	}

	// Start session
	s := session.New(opt.Spool, 0, true, nil, config.SessionInspectorsConfig{})
	defer s.Discard()

	// Create a fake artifact
	sha, _ := digests.NewSha256Digest("5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03")
	a1 := fakeArtifact(sha, s, c)

	// We're the service dispatcher.
	ch := make(chan interface{})
	go func(ch chan interface{}) {
		// Receive the a1 inspection message.
		msg1 := <-ch
		v1 := msg1.(messages.ResponseInspection)
		service.HandleResponseInspection(v1, ch)

		// Receive the end of a1 inspection message.
		msg2 := <-ch
		v2 := msg2.(messages.CompleteInspection)

		// Receive the a2 inspection message.
		msg3 := <-ch
		v3 := msg3.(messages.ResponseInspection)
		service.HandleResponseInspection(v3, ch)

		// Receive the end of a2 inspection message.
		msg4 := <-ch
		v4 := msg4.(messages.CompleteInspection)
		service.HandleCompleteInspection(v4)

		// Receive the a3 inspection message.
		msg5 := <-ch
		v5 := msg5.(messages.ResponseInspection)
		service.HandleResponseInspection(v5, ch)

		// There's no inspection completion message for a3 because the a2
		// result was reused. Receive the status request instead.
		msg6 := <-ch
		_ = msg6.(messages.GetServiceStatus)

		// Finally complete the a1 inspection
		service.HandleCompleteInspection(v2)
	}(ch)

	// Request inspection
	insp1 := messages.NewResponseInspection(a1)
	ch <- insp1

	time.Sleep(2 * time.Second) // Wait for inspectors to run

	// The artifact was not added to the session because inspection is not complete.
	c.Assert(s.HasArtifact(sha), Equals, false)

	// Inspect another download of the same artifact and request inspection
	a2 := fakeArtifact(sha, s, c)
	insp2 := messages.NewResponseInspection(a2)
	ch <- insp2

	// Collect the result of the a2 inspection, a1 still pending
	err = <-insp2.Rch
	c.Assert(err, IsNil)

	// The artifact was now added to the session because the a2 inspection is complete,
	// and only one download is listed.
	c.Assert(s.HasArtifact(sha), Equals, true)
	c.Assert(s.A[sha].Downloads, HasLen, 1)

	// Inspect another download of the same artifact, the a2 inspection result is reused
	a3 := fakeArtifact(sha, s, c)
	insp3 := messages.NewResponseInspection(a3)
	ch <- insp3

	// Collect the result of the a3 inspection, a1 still pending
	err = <-insp3.Rch
	c.Assert(err, IsNil)

	// The a3 download was added to the artifact.
	c.Assert(s.A[sha].Downloads, HasLen, 2)

	// Send a different message to be read by the service.
	status1 := messages.NewGetServiceStatus()
	ch <- status1

	// Finally receive the a1 inspection result.
	err = <-insp1.Rch
	c.Assert(err, IsNil)

	// All downloads added to the artifact.
	c.Assert(s.A[sha].Downloads, HasLen, 3)
}
