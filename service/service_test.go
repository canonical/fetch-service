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
	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
	"github.com/canonical/fetch-service/testutils"
	"github.com/canonical/fetch-service/version"
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

	restorer = service.MockNewControlServer(func(port int, ch chan interface{}, creds string) *control.Server {
		t.controlPort = port
		return &control.Server{}
	})
	defer restorer()

	svc, err := service.New(serviceOptionsFixture(c))
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

	restorer = service.MockNewControlServer(func(port int, ch chan interface{}, creds string) *control.Server {
		t.controlPort = port
		return &control.Server{}
	})
	defer restorer()

	_, err := service.New(serviceOptionsFixture(c))
	c.Assert(err, ErrorMatches, "proxy start error")
}

func (t *serviceSuite) TestConfigServerCrash(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	var cfg *config.Server
	restorer = service.MockNewConfigServer(func(ch chan interface{}) *config.Server {
		cfg = config.NewServer(ch)
		return cfg
	})
	defer restorer()

	svc, err := service.New(serviceOptionsFixture(c))
	c.Assert(err, IsNil)

	err = svc.Start()
	c.Assert(err, IsNil)
	c.Assert(svc.Alive(), Equals, true)

	err = cfg.Stop() // config server crashes
	c.Assert(err, IsNil)
	time.Sleep(2 * time.Second)
	c.Assert(svc.Alive(), Equals, false)
}

func (t *serviceSuite) TestControlServerCrash(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	var ctl *control.Server
	restorer = service.MockNewControlServer(func(port int, ch chan interface{}, creds string) *control.Server {
		ctl = control.NewServer(port, ch, creds)
		return ctl
	})
	defer restorer()

	svc, err := service.New(serviceOptionsFixture(c))
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

	svc, err := service.New(serviceOptionsFixture(c))
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

	svc, err := service.New(serviceOptionsFixture(c))
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

	dir := c.MkDir()
	certPath, keyPath, err := createCertFiles(dir)
	c.Assert(err, IsNil)

	for _, tc := range []struct {
		createSession bool
		serviceAlive  bool
	}{
		{false, false},
		{true, true},
	} {
		opt := service.Options{
			ProxyPort:      1337,
			IdleShutdown:   1,
			PermissiveMode: true,
			Spool:          dir,
			CertPath:       certPath,
			KeyPath:        keyPath,
		}
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

	dir := c.MkDir()
	certPath, keyPath, err := createCertFiles(dir)
	c.Assert(err, IsNil)

	opt := service.Options{
		ProxyPort: 1337,
		Spool:     "/my/spool",
		CertPath:  certPath,
		KeyPath:   keyPath,
	}
	svc, err := service.New(&opt)
	c.Assert(err, IsNil)

	err = svc.Start()
	c.Assert(err, IsNil)
	s := session.New(opt.Spool, 0, true)
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

	dir := c.MkDir()
	certPath, keyPath, err := createCertFiles(dir)
	c.Assert(err, IsNil)

	opt := service.Options{
		ProxyPort: 1337,
		Spool:     "/my/spool",
		CertPath:  certPath,
		KeyPath:   keyPath,
	}

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
		s := session.New(opt.Spool, 0, true)
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

		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)
		s := session.New(opt.Spool, 0, true)
		defer s.Discard()

		sha, _ := digests.NewSha256Digest("5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03")
		a := metadata.NewArtefact()
		a.Metadata.Sha256 = sha
		a.AssetDir = filepath.Join(s.SessionDir, "assets")

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

	dir := c.MkDir()
	certPath, keyPath, err := createCertFiles(dir)
	c.Assert(err, IsNil)

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
			CertPath:       certPath,
			KeyPath:        keyPath,
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

		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)

		s := session.New(opt.Spool, 0, true)
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

	dir := c.MkDir()
	certPath, keyPath, err := createCertFiles(dir)
	c.Assert(err, IsNil)

	for _, tc := range []struct {
		sessionExists bool
		tokenIsValid  bool
		err           error
	}{
		{true, true, nil},
		{true, false, messages.ErrInvalidSessionToken},
		{false, true, messages.ErrSessionNotFound},
	} {
		opt := service.Options{
			ProxyPort: 1337,
			Spool:     "/my/spool",
			CertPath:  certPath,
			KeyPath:   keyPath,
		}
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)
		s := session.New(opt.Spool, 0, true)
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
		dir := c.MkDir()
		certPath, keyPath, err := createCertFiles(dir)
		c.Assert(err, IsNil)

		opt := service.Options{
			ProxyPort: 1337,
			Spool:     dir,
			CertPath:  certPath,
			KeyPath:   keyPath,
		}
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)
		s := session.New(opt.Spool, 0, true)
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
			c.Assert(res.SessionMetadata.SpoolPath, Equals, filepath.Join(dir, s.Id))
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

	dir := c.MkDir()
	certPath, keyPath, err := createCertFiles(dir)
	c.Assert(err, IsNil)

	for _, tc := range []struct {
		sessionExists bool
		tokenRevoked  bool
		err           error
	}{
		{true, true, nil},
		{true, false, nil},
		{false, true, messages.ErrSessionNotFound},
	} {
		opt := service.Options{
			ProxyPort: 1337,
			Spool:     dir,
			CertPath:  certPath,
			KeyPath:   keyPath,
		}
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)
		s := session.New(opt.Spool, 0, true)
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
	restorer := service.MockNewControlServer(func(port int, ch chan interface{}, creds string) *control.Server {
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
	_, err := service.New(serviceOptionsFixture(c))
	c.Assert(err, IsNil)
	c.Assert(t.controlAuth, Equals, "suzy:shalamacookie")
}

func (t *serviceSuite) TestConfiguration(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.ch = ch
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	dir := c.MkDir()
	certPath, keyPath, err := createCertFiles(dir)
	c.Assert(err, IsNil)

	for _, tc := range []struct {
		operation string
		optype    string
		dryRun    bool
		cfgFail   bool
		result    string
		message   string
	}{
		{"version", "", false, false, "ok", version.Version},
		{"update-config", "foo", false, false, "ok", "configuration updated"},
		{"update-config", "foo", false, true, "error", "foo configuration update error"},
		{"update-config", "foo", true, false, "ok", "configuration validated"},
		{"", "", false, false, "error", "unsupported operation"},
		{"invalid", "", false, false, "error", "unsupported operation"},
	} {
		restorer = service.MockConfigUpdateConfig(func(optype string, dryRun bool, payload []byte, cfgdir string) error {
			if tc.cfgFail {
				return errors.New("something failed")
			}
			return nil
		})
		defer restorer()

		spool := filepath.Join(dir, "spool")
		opt := service.Options{ProxyPort: 1337, Spool: spool, CertPath: certPath, KeyPath: keyPath}
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)
		s := session.New(opt.Spool, 0, true)
		defer s.Discard()

		msg := messages.NewConfiguration(tc.operation, tc.optype, tc.dryRun, nil)
		t.ch <- msg
		res := <-msg.Rch

		c.Assert(res.Status, Equals, tc.result)
		c.Assert(res.Message, Equals, tc.message)

		err = svc.Stop()
		c.Assert(err, IsNil)
	}
}

func createCertFiles(dir string) (string, string, error) {
	certPath := filepath.Join(dir, "cert")
	if err := os.WriteFile(certPath, testutils.ProxyCert, 0644); err != nil {
		return "", "", err
	}

	keyPath := filepath.Join(dir, "key")
	if err := os.WriteFile(keyPath, testutils.ProxyKey, 0644); err != nil {
		return "", "", err
	}

	return certPath, keyPath, nil
}

func serviceOptionsFixture(c *C) *service.Options {
	dir := c.MkDir()
	certPath, keyPath, err := createCertFiles(dir)
	c.Assert(err, IsNil)

	return &service.Options{
		ProxyPort:   1337,
		ControlPort: 7331,
		CertPath:    certPath,
		KeyPath:     keyPath,
	}
}
