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
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/control"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
	"github.com/canonical/fetch-service/testutils"
)

func Test(t *testing.T) { TestingT(t) }

type serviceSuite struct {
	ch          chan any
	proxyPort   int
	controlPort int
	controlAuth string
}

func (t *serviceSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&serviceSuite{})

// Check if the proxy and control API are created with the correct port number.
func (t *serviceSuite) TestProxyPort(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	restorer = service.MockNewServer(func(port int, ch chan interface{}, creds string) *control.Server {
		t.controlPort = port
		return &control.Server{}
	})
	defer restorer()

	opt := service.Options{ProxyPort: 1337, ControlPort: 7331}

	svc, err := service.New(&opt)
	c.Assert(err, IsNil)
	c.Assert(svc, FitsTypeOf, &service.Service{})
	c.Assert(t.proxyPort, Equals, 1337)
	c.Assert(t.controlPort, Equals, 7331)
}

func (t *serviceSuite) TestProxyStartError(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		return nil, errors.New("proxy start error")
	})
	defer restorer()

	restorer = service.MockNewServer(func(port int, ch chan interface{}, creds string) *control.Server {
		t.controlPort = port
		return &control.Server{}
	})
	defer restorer()

	opt := service.Options{ProxyPort: 1337, ControlPort: 7331}

	_, err := service.New(&opt)
	c.Assert(err, ErrorMatches, "proxy start error")
}

func (t *serviceSuite) TestControlServerCrash(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	var ctl *control.Server
	restorer = service.MockNewServer(func(port int, ch chan interface{}, creds string) *control.Server {
		ctl = control.NewServer(port, ch, creds)
		return ctl
	})
	defer restorer()

	opt := service.Options{ProxyPort: 1337, ControlPort: 7331}

	svc, err := service.New(&opt)
	c.Assert(err, IsNil)

	err = svc.Start()
	c.Assert(err, IsNil)
	c.Assert(svc.Alive(), Equals, true)

	err = ctl.Stop() // control server crashes
	c.Assert(err, IsNil)
	time.Sleep(2 * time.Second)
	c.Assert(svc.Alive(), Equals, false)
}

func (t *serviceSuite) TestHttpProxyCrash(c *C) {
	var px *proxy.HttpProxy
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		var err error
		px, err = proxy.NewHttpProxy(port, spool, cert, key, ch)
		return px, err
	})
	defer restorer()

	opt := service.Options{
		ProxyPort:   1337,
		ControlPort: 7331,
		Cert:        testutils.ProxyCert,
		Key:         testutils.ProxyKey,
	}

	svc, err := service.New(&opt)
	c.Assert(err, IsNil)

	err = svc.Start()
	c.Assert(err, IsNil)
	c.Assert(svc.Alive(), Equals, true)

	err = px.Stop() // proxy crashes
	c.Assert(err, IsNil)
	time.Sleep(2 * time.Second)
	c.Assert(svc.Alive(), Equals, false)
}

func (t *serviceSuite) TestServiceEntombment(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	opt := service.Options{ProxyPort: 1337}
	svc, err := service.New(&opt)
	c.Assert(err, IsNil)

	err = svc.Start()
	c.Assert(err, IsNil)

	c.Assert(svc.Alive(), Equals, true)

	err = svc.Stop()
	c.Assert(err, IsNil)

	c.Assert(svc.Alive(), Equals, false)
}

func (t *serviceSuite) TestServiceIdleShutdown(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.ch = ch
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	for _, tc := range []struct {
		createSession bool
		serviceAlive  bool
	}{
		{false, false},
		{true, true},
	} {

		opt := service.Options{ProxyPort: 1337, IdleShutdown: 1, PermissiveMode: true}
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)

		var sid string
		if tc.createSession {
			msg := messages.NewCreateSession("permissive", 1338)
			t.ch <- msg
			res := <-msg.Rch
			c.Assert(res.Err, Equals, nil)
			sid = res.Id
		}

		c.Assert(svc.Alive(), Equals, true)
		time.Sleep(2 * time.Second)
		c.Assert(svc.Alive(), Equals, tc.serviceAlive)

		if !tc.createSession {
			continue
		}

		msg := messages.NewEndSession(sid)
		t.ch <- msg
		err = <-msg.Rch
		c.Assert(err, IsNil)

		time.Sleep(2 * time.Second)
		c.Assert(svc.Alive(), Equals, false)
	}
}

func (t *serviceSuite) TestGetServiceStatus(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.ch = ch
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	opt := service.Options{ProxyPort: 1337, Spool: "/my/spool"}
	svc, err := service.New(&opt)
	c.Assert(err, IsNil)

	err = svc.Start()
	c.Assert(err, IsNil)
	s := session.New(opt.Spool, true)
	defer s.Discard()

	msg := messages.NewGetServiceStatus()
	t.ch <- msg
	res := <-msg.Rch

	c.Assert(len(res.ActiveSessions), Equals, 1)
	c.Check(res.ActiveSessions[0].SessionId, Not(Equals), "")
	c.Check(res.ActiveSessions[0].Policy, Equals, "permissive")
	c.Check(res.ActiveSessions[0].Timeout, Equals, uint64(6)*3600)

	err = svc.Stop()
	c.Assert(err, IsNil)
}

func (t *serviceSuite) TestRequestInspection(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.ch = ch
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	opt := service.Options{ProxyPort: 1337, Spool: "/my/spool"}

	for _, tc := range []struct {
		sessionExists bool
		errMsg        string
	}{
		{true, ""},
		{false, "cannot inspect request: session foo is not active"},
	} {
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)
		s := session.New(opt.Spool, true)
		defer s.Discard()

		a := metadata.NewArtefact()
		if tc.sessionExists {
			a.SessionId = s.Id
		} else {
			a.SessionId = "foo"
		}
		msg := messages.NewRequestInspection(a)
		t.ch <- msg
		res := <-msg.Rch

		if tc.errMsg == "" {
			c.Assert(res, Equals, nil)
		} else {
			c.Assert(res, ErrorMatches, tc.errMsg)
		}

		err = svc.Stop()
		c.Assert(err, IsNil)
	}
}

func (t *serviceSuite) TestResponseInspection(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.ch = ch
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	for _, tc := range []struct {
		sessionExists bool
		hasArtefact   bool
		errMsg        string
	}{
		{true, false, ""},
		{true, true, ""},
		{false, false, "cannot inspect response: session foo is not active"},
	} {
		spoolDir := c.MkDir()
		opt := service.Options{ProxyPort: 1337, Spool: spoolDir}

		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)
		s := session.New(opt.Spool, true)
		defer s.Discard()

		sha, _ := digests.NewSha256Digest("5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03")
		a := metadata.NewArtefact()
		a.Metadata.Sha256 = sha

		tmpfile := filepath.Join(spoolDir, "tempfile")
		err = os.WriteFile(tmpfile, []byte("content"), 0644)
		c.Assert(err, IsNil)

		a.Tempfile = tmpfile
		if tc.hasArtefact { // this sha has already been downloaded
			s.A[sha] = a
		}
		if tc.sessionExists {
			a.SessionId = s.Id
		} else {
			a.SessionId = "foo"
		}
		msg := messages.NewResponseInspection(a)
		t.ch <- msg
		res := <-msg.Rch

		if tc.errMsg == "" {
			c.Assert(res, Equals, nil)
		} else {
			c.Assert(res, ErrorMatches, tc.errMsg)
		}

		err = svc.Stop()
		c.Assert(err, IsNil)
	}
}

func (t *serviceSuite) TestCreateSession(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.ch = ch
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	for _, tc := range []struct {
		permissiveMode bool
		policy         string
		errMsg         string
	}{
		{true, "permissive", ""},
		{false, "permissive", "Invalid session policy"},
		{false, "strict", ""},
	} {
		opt := service.Options{
			ProxyPort:      1337,
			Spool:          "/my/spool",
			PermissiveMode: tc.permissiveMode,
		}
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)

		msg := messages.NewCreateSession(tc.policy, 666)
		t.ch <- msg
		res := <-msg.Rch

		if tc.errMsg == "" {
			c.Assert(res.Err, Equals, nil)
			s := session.GetSession(res.Id)
			c.Assert(s.Permissive, Equals, tc.policy == "permissive")
			s.Discard()
		} else {
			c.Assert(res.Err, ErrorMatches, tc.errMsg)
		}

		err = svc.Stop()
		c.Assert(err, IsNil)
	}
}

func (t *serviceSuite) TestDeleteResources(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.ch = ch
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	for _, tc := range []struct {
		sessionExists bool
		errMsg        string
	}{
		{false, ""},
		{true, "session not finished"},
	} {
		spoolDir := c.MkDir()
		opt := service.Options{ProxyPort: 1337, Spool: spoolDir}

		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)

		s := session.New(opt.Spool, true)
		defer s.Discard()

		var sid string
		if tc.sessionExists {
			sid = s.Id
		} else {
			sid = "other value"
		}

		msg := messages.NewDeleteResources(sid)
		t.ch <- msg
		res := <-msg.Rch

		if tc.errMsg == "" {
			c.Assert(res, Equals, nil)
		} else {
			c.Assert(res, ErrorMatches, tc.errMsg)
		}

		err = svc.Stop()
		c.Assert(err, IsNil)
	}
}

func (t *serviceSuite) TestRevokeToken(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.ch = ch
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	for _, tc := range []struct {
		sessionExists bool
		tokenIsValid  bool
		err           error
	}{
		{true, true, nil},
		{true, false, messages.ErrInvalidSessionToken},
		{false, true, messages.ErrSessionNotFound},
	} {
		opt := service.Options{ProxyPort: 1337, Spool: "/my/spool"}
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)
		s := session.New(opt.Spool, true)
		defer s.Discard()

		var sid string
		if tc.sessionExists {
			sid = s.Id
		} else {
			sid = "other value"
		}

		var token string
		if tc.tokenIsValid {
			token = s.Token
		} else {
			token = "invalid-token"
		}

		msg := messages.NewRevokeToken(sid, token)
		t.ch <- msg
		res := <-msg.Rch

		if tc.err == nil {
			c.Assert(res.Err, IsNil)
			c.Assert(res.SpoolPath, Equals, "/my/spool")
			c.Assert(res.SessionId, Equals, s.Id)
		} else {
			c.Assert(res.Err, Equals, tc.err)
		}

		err = svc.Stop()
		c.Assert(err, IsNil)
	}
}

func (t *serviceSuite) TestGetSessionReport(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.ch = ch
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	for _, tc := range []struct {
		sessionExists bool
		tokenRevoked  bool
		err           error
	}{
		{true, true, nil},
		{true, false, messages.ErrSessionActive},
		{false, true, messages.ErrSessionNotFound},
	} {
		spool := c.MkDir()
		opt := service.Options{ProxyPort: 1337, Spool: spool}
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)
		s := session.New(opt.Spool, true)
		defer s.Discard()

		var sid string
		if tc.sessionExists {
			sid = s.Id
		} else {
			sid = "other value"
		}

		if tc.tokenRevoked {
			s.Revoke(s.Token)
		}

		msg := messages.NewSessionReport(sid)
		t.ch <- msg
		res := <-msg.Rch

		if tc.err == nil {
			c.Assert(res.Err, IsNil)
			c.Assert(res.Artefacts, DeepEquals, []*metadata.Artefact{})
			c.Assert(res.SessionMetadata.SessionId, Equals, s.Id)
			c.Assert(res.SessionMetadata.SpoolPath, Equals, filepath.Join(spool, s.Id))
		} else {
			c.Assert(res.Err, Equals, tc.err)
		}

		err = svc.Stop()
		c.Assert(err, IsNil)
	}
}

func (t *serviceSuite) TestEndSession(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.ch = ch
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	for _, tc := range []struct {
		sessionExists bool
		tokenRevoked  bool
		err           error
	}{
		{true, true, nil},
		{true, false, nil},
		{false, true, messages.ErrSessionNotFound},
	} {
		spool := c.MkDir()
		opt := service.Options{ProxyPort: 1337, Spool: spool}
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)
		s := session.New(opt.Spool, true)
		defer s.Discard()

		var sid string
		if tc.sessionExists {
			sid = s.Id
		} else {
			sid = "other value"
		}

		if tc.tokenRevoked {
			s.Revoke(s.Token)
		}

		msg := messages.NewEndSession(sid)
		t.ch <- msg
		err = <-msg.Rch

		c.Assert(err, Equals, tc.err)

		err = svc.Stop()
		c.Assert(err, IsNil)
	}
}

func (t *serviceSuite) TestControlAuthentication(c *C) {
	restorer := service.MockNewServer(func(port int, ch chan interface{}, creds string) *control.Server {
		t.controlPort = port
		t.controlAuth = creds
		return &control.Server{}
	})
	defer restorer()

	restorer = service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.ch = ch
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	os.Setenv("FETCH_SERVICE_AUTH", "suzy:shalamacookie")
	opt := service.Options{}
	_, err := service.New(&opt)
	c.Assert(err, IsNil)
	c.Assert(t.controlAuth, Equals, "suzy:shalamacookie")
}
