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

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/service/messages"
)

type messagesSuite struct {
}

var _ = Suite(&messagesSuite{})

func Test(t *testing.T) { TestingT(t) }

func (t *messagesSuite) TestGetServiceStatus(c *C) {
	var m messages.GetServiceStatus
	m = messages.NewGetServiceStatus()
	c.Check(cap(m.Rch), Equals, 1)
}

func (t *messagesSuite) TestRequestInspection(c *C) {
	a := metadata.NewArtifact()
	var m messages.RequestInspection
	m = messages.NewRequestInspection(a)
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.A, Equals, a)
}

func (t *messagesSuite) TestResponseInspection(c *C) {
	a := metadata.NewArtifact()
	var m messages.ResponseInspection
	m = messages.NewResponseInspection(a)
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.A, Equals, a)
}

func (t *messagesSuite) TestCompleteInspection(c *C) {
	a := metadata.NewArtifact()
	var m messages.CompleteInspection
	m = messages.NewCompleteInspection(a)
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.A, Equals, a)
}

func (t *messagesSuite) TestCreateSession(c *C) {
	var m messages.CreateSession
	m = messages.NewCreateSession("policy", 42)
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.Policy, Equals, "policy")
	c.Check(m.Timeout, Equals, uint64(42))
}

func (t *messagesSuite) TestRevokeToken(c *C) {
	var m messages.RevokeToken
	m = messages.NewRevokeToken("session-id", "token")
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.Id, Equals, "session-id")
	c.Check(m.Token, Equals, "token")
}

func (t *messagesSuite) TestSessionReport(c *C) {
	var m messages.SessionReport
	m = messages.NewSessionReport("session-id")
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.Id, Equals, "session-id")
}

func (t *messagesSuite) TestEndSession(c *C) {
	var m messages.EndSession
	m = messages.NewEndSession("session-id")
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.Id, Equals, "session-id")
}

func (t *messagesSuite) TestDeleteResources(c *C) {
	var m messages.DeleteResources
	m = messages.NewDeleteResources("session-id")
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.Id, Equals, "session-id")
}

func (t *messagesSuite) TestFetchCtl(c *C) {
	var m messages.FetchCtl
	m = messages.NewFetchCtl("operation", "type", true, []byte("payload"))
	c.Check(cap(m.Rch), Equals, 1)
	c.Check(m.Operation, Equals, "operation")
	c.Check(m.Type, Equals, "type")
	c.Check(m.ValidateOnly, Equals, true)
	c.Check(m.Payload, DeepEquals, []byte("payload"))
}
