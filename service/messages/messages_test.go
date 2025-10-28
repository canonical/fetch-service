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

package messages_test

import (
	"testing"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/secrets"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/service/messages"
)

type messagesSuite struct {
}

var _ = Suite(&messagesSuite{})

func Test(t *testing.T) { TestingT(t) }

func (t *messagesSuite) TestGetServiceStatus(c *C) {
	var m = messages.NewGetServiceStatus()
	c.Check(cap(m.Rch), Equals, 1)
}

func (t *messagesSuite) TestRequestInspection(c *C) {
	a := metadata.NewArtifact()
	var m = messages.NewRequestInspection(a)
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.A, Equals, a)
}

func (t *messagesSuite) TestResponseInspection(c *C) {
	a := metadata.NewArtifact()
	var m = messages.NewResponseInspection(a)
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.A, Equals, a)
}

func (t *messagesSuite) TestCompleteInspection(c *C) {
	a := metadata.NewArtifact()
	var m = messages.NewCompleteInspection(a)
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.A, Equals, a)
}

func (t *messagesSuite) TestCreateSession(c *C) {
	var m = messages.NewCreateSession("policy", 42, nil, config.SessionInspectorsConfig{})
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.Policy, Equals, "policy")
	c.Check(m.Timeout, Equals, uint64(42))
	c.Check(len(m.Secrets), Equals, 0)
}

func (t *messagesSuite) TestCreateSessionWithSecrets(c *C) {
	s := []secrets.Secret{
		{Type: secrets.BasicAuthType, Url: glob.MustCompile("http://example.com")},
		{Type: secrets.BasicAuthType, Url: glob.MustCompile("http://another-example.com/*")},
	}
	var m = messages.NewCreateSession("policy", 42, s, config.SessionInspectorsConfig{})
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.Policy, Equals, "policy")
	c.Check(m.Timeout, Equals, uint64(42))
	c.Check(len(m.Secrets), Equals, 2)
	c.Check(m.Secrets, DeepEquals, s)
}

func (t *messagesSuite) TestRevokeToken(c *C) {
	var m = messages.NewRevokeToken("session-id", "token")
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.ID, Equals, "session-id")
	c.Check(m.Token, Equals, "token")
}

func (t *messagesSuite) TestSessionReport(c *C) {
	var m = messages.NewSessionReport("session-id")
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.ID, Equals, "session-id")
}

func (t *messagesSuite) TestEndSession(c *C) {
	var m = messages.NewEndSession("session-id")
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.ID, Equals, "session-id")
}

func (t *messagesSuite) TestDeleteResources(c *C) {
	var m = messages.NewDeleteResources("session-id")
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.ID, Equals, "session-id")
}

func (t *messagesSuite) TestFetchCtl(c *C) {
	var m = messages.NewFetchCtl("operation", "type", true, []byte("payload"))
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.Operation, Equals, "operation")
	c.Check(m.Type, Equals, "type")
	c.Check(m.ValidateOnly, Equals, true)
	c.Check(m.Payload, DeepEquals, []byte("payload"))
}
