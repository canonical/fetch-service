// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2024 Canonical Ltd.
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
	"path/filepath"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
)

func Test(t *testing.T) { TestingT(t) }

type serviceSuite struct {
	ch   chan any
	port int
}

func (t *serviceSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&serviceSuite{})

// Check if the proxy is created with the correct port number.
func (t *serviceSuite) TestProxyPort(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, ch chan interface{}) *proxy.HttpProxy {
		t.port = port
		return &proxy.HttpProxy{}
	})
	defer restorer()

	opt := service.Options{ProxyPort: 1337}

	svc := service.New(&opt)
	c.Assert(svc, FitsTypeOf, &service.Service{})
	c.Assert(t.port, Equals, 1337)
}

func (t *serviceSuite) TestServiceEntombment(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, ch chan interface{}) *proxy.HttpProxy {
		t.port = port
		return &proxy.HttpProxy{}
	})
	defer restorer()

	opt := service.Options{ProxyPort: 1337}
	svc := service.New(&opt)

	err := svc.Start()
	c.Assert(err, IsNil)

	err = svc.Stop()
	c.Assert(err, IsNil)
}

func (t *serviceSuite) TestRevokeToken(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, ch chan interface{}) *proxy.HttpProxy {
		t.ch = ch
		t.port = port
		return &proxy.HttpProxy{}
	})
	defer restorer()

	opt := service.Options{ProxyPort: 1337, Spool: "/my/spool"}
	svc := service.New(&opt)

	err := svc.Start()
	c.Assert(err, IsNil)
	s := session.New(opt.Spool, true)
	defer s.Discard()

	msg := messages.NewRevokeToken(s.Id)
	t.ch <- msg
	res := <-msg.Rch

	c.Assert(res.Err, IsNil)
	c.Assert(res.SpoolPath, Equals, "/my/spool")
	c.Assert(res.SessionId, Equals, s.Id)

	err = svc.Stop()
	c.Assert(err, IsNil)
}

func (t *serviceSuite) TestRevokeTokenInvalidSession(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, ch chan interface{}) *proxy.HttpProxy {
		t.ch = ch
		t.port = port
		return &proxy.HttpProxy{}
	})
	defer restorer()

	opt := service.Options{ProxyPort: 1337, Spool: "/my/spool"}
	svc := service.New(&opt)

	err := svc.Start()
	c.Assert(err, IsNil)

	msg := messages.NewRevokeToken("invalid-session")
	t.ch <- msg
	res := <-msg.Rch

	c.Assert(res.Err.Error(), Equals, "session not found")

	err = svc.Stop()
	c.Assert(err, IsNil)
}

func (t *serviceSuite) TestGetSessionReport(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, ch chan interface{}) *proxy.HttpProxy {
		t.ch = ch
		t.port = port
		return &proxy.HttpProxy{}
	})
	defer restorer()

	spool := c.MkDir()
	opt := service.Options{ProxyPort: 1337, Spool: spool}
	svc := service.New(&opt)

	err := svc.Start()
	c.Assert(err, IsNil)
	s := session.New(opt.Spool, true)
	defer s.Discard()

	s.Revoke()

	msg := messages.NewSessionReport(s.Id)
	t.ch <- msg
	res := <-msg.Rch

	c.Assert(res.Err, IsNil)
	c.Assert(res.Artefacts, DeepEquals, []*metadata.Artefact{})
	c.Assert(res.SessionMetadata.SessionId, Equals, s.Id)
	c.Assert(res.SessionMetadata.SpoolPath, Equals, filepath.Join(spool, s.Id))

	err = svc.Stop()
	c.Assert(err, IsNil)
}

func (t *serviceSuite) TestGetSessionReportNotRevoked(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, ch chan interface{}) *proxy.HttpProxy {
		t.ch = ch
		t.port = port
		return &proxy.HttpProxy{}
	})
	defer restorer()

	spool := c.MkDir()
	opt := service.Options{ProxyPort: 1337, Spool: spool}
	svc := service.New(&opt)

	err := svc.Start()
	c.Assert(err, IsNil)
	s := session.New(opt.Spool, true)
	defer s.Discard()

	// session not revoked

	msg := messages.NewSessionReport(s.Id)
	t.ch <- msg
	res := <-msg.Rch

	c.Assert(res.Err.Error(), Equals, "session token is active")

	err = svc.Stop()
	c.Assert(err, IsNil)
}

func (t *serviceSuite) TestGetSessionReportInvalidSession(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, ch chan interface{}) *proxy.HttpProxy {
		t.ch = ch
		t.port = port
		return &proxy.HttpProxy{}
	})
	defer restorer()

	spool := c.MkDir()
	opt := service.Options{ProxyPort: 1337, Spool: spool}
	svc := service.New(&opt)

	err := svc.Start()
	c.Assert(err, IsNil)
	s := session.New(opt.Spool, true)
	defer s.Discard()

	s.Revoke()

	msg := messages.NewSessionReport("invalid-session")
	t.ch <- msg
	res := <-msg.Rch

	c.Assert(res.Err.Error(), Equals, "session not found")

	err = svc.Stop()
	c.Assert(err, IsNil)
}

func (t *serviceSuite) TestEndSession(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, ch chan interface{}) *proxy.HttpProxy {
		t.ch = ch
		t.port = port
		return &proxy.HttpProxy{}
	})
	defer restorer()

	spool := c.MkDir()
	opt := service.Options{ProxyPort: 1337, Spool: spool}
	svc := service.New(&opt)

	err := svc.Start()
	c.Assert(err, IsNil)
	s := session.New(opt.Spool, true)
	defer s.Discard()

	msg := messages.NewEndSession(s.Id)
	t.ch <- msg
	err = <-msg.Rch

	c.Assert(err, IsNil)

	err = svc.Stop()
	c.Assert(err, IsNil)
}

func (t *serviceSuite) TestEndSessionInvalidSession(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, ch chan interface{}) *proxy.HttpProxy {
		t.ch = ch
		t.port = port
		return &proxy.HttpProxy{}
	})
	defer restorer()

	spool := c.MkDir()
	opt := service.Options{ProxyPort: 1337, Spool: spool}
	svc := service.New(&opt)

	err := svc.Start()
	c.Assert(err, IsNil)
	s := session.New(opt.Spool, true)
	defer s.Discard()

	msg := messages.NewEndSession("invalid-session")
	t.ch <- msg
	err = <-msg.Rch

	c.Assert(err.Error(), Equals, "session not found")

	err = svc.Stop()
	c.Assert(err, IsNil)
}
