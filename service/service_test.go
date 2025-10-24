// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2023-2025 Canonical Ltd.
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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/secrets"
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/control"
	. "github.com/canonical/fetch-service/inspectors/common"
	"github.com/canonical/fetch-service/metadata"
	"github.com/canonical/fetch-service/metadata/digests"
	"github.com/canonical/fetch-service/metadata/opinions"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service"
	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/service/fetchctl"
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
}

func (t *serviceSuite) TearDownTest(c *C) {
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

func (t *serviceSuite) TestFetchctlServerCrash(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	var fctl *fetchctl.Server
	restorer = service.MockNewFetchctlServer(func(ch chan interface{}) *fetchctl.Server {
		fctl = fetchctl.NewServer(ch)
		return fctl
	})
	defer restorer()

	svc, err := service.New(serviceOptionsFixture(c))
	c.Assert(err, IsNil)

	err = svc.Start()
	c.Assert(err, IsNil)
	c.Assert(svc.Alive(), Equals, true)

	err = fctl.Stop() // config server crashes
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
	defer svc.Stop() // nolint:errcheck

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

	dir := c.MkDir()

	certPath := filepath.Join(dir, "cert")
	err := os.WriteFile(certPath, testutils.ProxyCert, 0644)
	c.Assert(err, IsNil)

	keyPath := filepath.Join(dir, "key")
	err = os.WriteFile(keyPath, testutils.ProxyKey, 0644)
	c.Assert(err, IsNil)

	opt := service.Options{
		ProxyPort:   1337,
		ControlPort: 7331,
		CertPath:    certPath,
		KeyPath:     keyPath,
	}

	svc, err := service.New(&opt)
	c.Assert(err, IsNil)

	err = svc.Start()
	c.Assert(err, IsNil)
	c.Assert(svc.Alive(), Equals, true)
	defer svc.Stop() // nolint:errcheck

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
			msg := messages.NewCreateSession("permissive", 1338, nil, config.SessionInspectorsConfig{})
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
		Spool:     dir,
		CertPath:  certPath,
		KeyPath:   keyPath,
	}
	svc, err := service.New(&opt)
	c.Assert(err, IsNil)

	err = svc.Start()
	c.Assert(err, IsNil)
	s := session.New(opt.Spool, 0, true, nil, config.SessionInspectorsConfig{})
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
		Spool:     dir,
		CertPath:  certPath,
		KeyPath:   keyPath,
	}

	for _, tc := range []struct {
		sessionExists bool
		policy        string
		errMsg        string
	}{
		{true, "permissive", ""},
		{false, "permissive", "cannot inspect request: session foo is not active"},
		{true, "strict", "request rejected by inspectors"},
		{false, "strict", "cannot inspect request: session foo is not active"},
	} {
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)
		s := session.New(opt.Spool, 0, tc.policy == "permissive", nil, config.SessionInspectorsConfig{})
		defer s.Discard()

		a := metadata.NewArtifact()
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

func (t *serviceSuite) TestEvaluateRequestInspection(c *C) {
	for _, tc := range []struct {
		policy        string
		inspections   metadata.InspectionMap
		expectedError error
	}{
		{"permissive", metadata.InspectionMap{}, nil},
		{"permissive", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Unknown}}, nil},
		{"permissive", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Rejected}}, nil},
		{"permissive", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Pending}}, nil},
		{"permissive", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Pending}, "bar": &Inspection{Opinion: opinions.Unknown}}, nil},
		{"permissive", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Pending}, "bar": &Inspection{Opinion: opinions.Rejected}}, nil},
		{"permissive", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Rejected}, "bar": &Inspection{Opinion: opinions.Unknown}}, nil},
		{"strict", metadata.InspectionMap{}, ErrRejectedRequest},
		{"strict", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Unknown}}, ErrRejectedRequest},
		{"strict", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Rejected}}, ErrRejectedRequest},
		{"strict", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Pending}}, nil},
		{"strict", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Pending}, "bar": &Inspection{Opinion: opinions.Unknown}}, nil},
		{"strict", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Pending}, "bar": &Inspection{Opinion: opinions.Rejected}}, ErrRejectedRequest},
		{"strict", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Rejected}, "bar": &Inspection{Opinion: opinions.Unknown}}, ErrRejectedRequest},
	} {
		s := session.New("/my/spool", 0, tc.policy == "permissive", nil, config.SessionInspectorsConfig{})
		defer s.Discard()

		a := metadata.NewArtifact()
		a.SessionId = s.Id
		a.RequestInspection = tc.inspections
		res := service.EvaluateRequestInspection(s, a)
		c.Assert(res, Equals, tc.expectedError)
	}
}

type responseInspectionTest struct {
	sessionExists bool   // Whether the session was created
	hasArtifact   bool   // Whether the artifact has already been downloaded
	policy        string // The inspection policy (strict or permissive)
	errMsg        string // The expected error message, if any
}

var responseInspectionTests = []responseInspectionTest{{
	sessionExists: true,
	hasArtifact:   false,
	policy:        "permissive",
	errMsg:        "",
}, {
	sessionExists: true,
	hasArtifact:   true,
	policy:        "permissive",
	errMsg:        "",
}, {
	sessionExists: false,
	hasArtifact:   false,
	policy:        "permissive",
	errMsg:        "cannot inspect response: session foo is not active",
}, {
	sessionExists: true,
	hasArtifact:   false,
	policy:        "strict",
	errMsg:        "artifact rejected by inspectors",
}, {
	sessionExists: true,
	hasArtifact:   true,
	policy:        "strict",
	errMsg:        "", // artifact has already been downloaded
}, {
	sessionExists: false,
	hasArtifact:   false,
	policy:        "strict",
	errMsg:        "cannot inspect response: session foo is not active",
}}

func (t *serviceSuite) TestResponseInspection(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, cert, key []byte, ch chan interface{}) (*proxy.HttpProxy, error) {
		t.ch = ch
		t.proxyPort = port
		return &proxy.HttpProxy{}, nil
	})
	defer restorer()

	for _, tc := range responseInspectionTests {
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
		s := session.New(opt.Spool, 0, tc.policy == "permissive", nil, config.SessionInspectorsConfig{})
		defer s.Discard()

		sha, _ := digests.NewSha256Digest("5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03")
		a := metadata.NewArtifact()
		a.Metadata.Sha256 = sha
		a.AssetDir = filepath.Join(s.SessionDir, "assets")

		tmpfile := filepath.Join(spoolDir, "tempfile")
		err = os.WriteFile(tmpfile, []byte("content"), 0644)
		c.Assert(err, IsNil)

		a.Tempfile = tmpfile
		if tc.hasArtifact { // this sha has already been downloaded
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

		// check if temporary file properly deleted
		time.Sleep(500 * time.Millisecond) // GitHub CI needs this
		_, err = os.Stat(a.Tempfile)
		c.Assert(err, ErrorMatches, "stat.*no such file or directory", Commentf("test case: %+v", tc))

		if tc.errMsg == "" {
			c.Assert(res, IsNil)
		} else {
			c.Assert(res, ErrorMatches, tc.errMsg, Commentf("test case: %+v", tc))
		}

		err = svc.Stop()
		c.Assert(err, IsNil)
	}
}

func fakeArtifact(sha digests.Sha256Digest, s *session.Session, c *C) *metadata.Artifact {
	a := metadata.NewArtifact()
	a.Metadata.Sha256 = sha
	a.AssetDir = filepath.Join(s.SessionDir, "assets")
	a.CurrentDownload = metadata.Download{URL: "http://d1", Sha256: sha}

	tmpfile, err := os.CreateTemp("", "tempfile-*")
	c.Assert(err, IsNil)
	defer tmpfile.Close()

	_, err = tmpfile.Write([]byte("content"))
	c.Assert(err, IsNil)

	a.Tempfile = tmpfile.Name()
	a.SessionId = s.Id
	return a
}

func (t *serviceSuite) TestResponseInspectionConcurrent(c *C) {
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

	svc, err := service.New(&opt)
	c.Assert(err, IsNil)

	err = svc.Start()
	c.Assert(err, IsNil)
	s := session.New(opt.Spool, 0, true, nil, config.SessionInspectorsConfig{})
	defer s.Discard()

	sha, _ := digests.NewSha256Digest("5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03")

	// First artifact download
	a1 := fakeArtifact(sha, s, c)

	// Second artifact download
	a2 := fakeArtifact(sha, s, c)

	msg1 := messages.NewResponseInspection(a1)
	t.ch <- msg1

	msg2 := messages.NewResponseInspection(a2)
	t.ch <- msg2

	res1 := <-msg1.Rch
	c.Assert(res1, IsNil)

	res2 := <-msg2.Rch
	c.Assert(res2, IsNil)

	// check if temporary files properly deleted
	time.Sleep(500 * time.Millisecond) // GitHub CI needs this

	_, err = os.Stat(a1.Tempfile)
	c.Assert(err, ErrorMatches, "stat.*no such file or directory")

	_, err = os.Stat(a2.Tempfile)
	c.Assert(err, ErrorMatches, "stat.*no such file or directory")

	// Check session data
	c.Assert(s.A, HasLen, 1) // Artifact is added once
	a := s.A[sha]
	c.Assert(a.Downloads, HasLen, 2)

	err = svc.Stop()
	c.Assert(err, IsNil)
}

func (t *serviceSuite) TestEvaluateResponseInspection(c *C) {
	for _, tc := range []struct {
		policy        string
		inspections   metadata.InspectionMap
		result        opinions.OpinionKind
		expectedError error
	}{
		{"permissive", metadata.InspectionMap{}, opinions.Rejected, nil},
		{"permissive", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Unknown}}, opinions.Rejected, nil},
		{"permissive", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Rejected}}, opinions.Rejected, nil},
		{"permissive", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Approved}}, opinions.Approved, nil},
		{"permissive", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Approved}, "bar": &Inspection{Opinion: opinions.Unknown}}, opinions.Approved, nil},
		{"permissive", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Approved}, "bar": &Inspection{Opinion: opinions.Rejected}}, opinions.Rejected, nil},
		{"permissive", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Rejected}, "bar": &Inspection{Opinion: opinions.Unknown}}, opinions.Rejected, nil},
		{"strict", metadata.InspectionMap{}, opinions.Rejected, ErrRejectedArtifact},
		{"strict", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Unknown}}, opinions.Rejected, ErrRejectedArtifact},
		{"strict", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Rejected}}, opinions.Rejected, ErrRejectedArtifact},
		{"strict", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Approved}}, opinions.Approved, nil},
		{"strict", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Approved}, "bar": &Inspection{Opinion: opinions.Unknown}}, opinions.Approved, nil},
		{"strict", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Approved}, "bar": &Inspection{Opinion: opinions.Rejected}}, opinions.Rejected, ErrRejectedArtifact},
		{"strict", metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Rejected}, "bar": &Inspection{Opinion: opinions.Unknown}}, opinions.Rejected, ErrRejectedArtifact},
	} {
		s := session.New("/my/spool", 0, tc.policy == "permissive", nil, config.SessionInspectorsConfig{})
		defer s.Discard()

		a := metadata.NewArtifact()
		a.SessionId = s.Id
		a.RequestInspection = metadata.InspectionMap{"foo": &Inspection{Opinion: opinions.Pending}}
		a.ResponseInspection = tc.inspections
		res := service.EvaluateResponseInspection(s, a)
		c.Assert(res, Equals, tc.expectedError)
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
		sec            []secrets.Secret
		errMsg         string
	}{
		{true, "permissive", nil, ""},
		{false, "permissive", nil, "Invalid session policy"},
		{false, "strict", nil, ""},
		// Cases for secrets
		// Good secret declaration
		{true, "permissive", []secrets.Secret{{Type: secrets.BasicAuthType, Url: glob.MustCompile("www.example.com"), BasicCreds: "user:passwd"}}, ""},
		// Missing type
		{true, "permissive", []secrets.Secret{{}}, secrets.ErrMissingSecretType.Error()},
		// Missing url
		{true, "permissive", []secrets.Secret{{Type: secrets.BasicAuthType}}, secrets.ErrMissingSecretUrl.Error()},
		// Invalid type
		{true, "permissive", []secrets.Secret{{Type: "invalid-type"}}, secrets.ErrInvalidSecretType.Error()},
		// Missing basic creds
		{true, "permissive", []secrets.Secret{{Type: secrets.BasicAuthType, Url: glob.MustCompile("www.example.com")}}, secrets.ErrMissingBasicCreds.Error()},
	} {
		opt := service.Options{
			ProxyPort:      1337,
			Spool:          dir,
			PermissiveMode: tc.permissiveMode,
			CertPath:       certPath,
			KeyPath:        keyPath,
		}
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)

		msg := messages.NewCreateSession(tc.policy, 666, tc.sec, config.SessionInspectorsConfig{})
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

		s := session.New(opt.Spool, 0, true, nil, config.SessionInspectorsConfig{})
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
			Spool:     dir,
			CertPath:  certPath,
			KeyPath:   keyPath,
		}
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)
		s := session.New(opt.Spool, 0, true, nil, config.SessionInspectorsConfig{})
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
			c.Assert(res.SpoolPath, Equals, dir)
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
		s := session.New(opt.Spool, 0, true, nil, config.SessionInspectorsConfig{})
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
			c.Assert(res.Artifacts, DeepEquals, []*metadata.Artifact{})
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
		s := session.New(opt.Spool, 0, true, nil, config.SessionInspectorsConfig{})
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

func (t *serviceSuite) TestFetchctlConfiguration(c *C) {
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
		s := session.New(opt.Spool, 0, true, nil, config.SessionInspectorsConfig{})
		defer s.Discard()

		msg := messages.NewFetchCtl(tc.operation, tc.optype, tc.dryRun, nil)
		t.ch <- msg
		res := <-msg.Rch

		c.Assert(res.Status, Equals, tc.result)
		c.Assert(res.Message, Equals, tc.message)

		err = svc.Stop()
		c.Assert(err, IsNil)
	}
}

func (t *serviceSuite) TestFetchctlCertificateUpdate(c *C) {
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
		dryRun  bool
		fail    bool
		result  string
		message string
	}{
		{false, false, "ok", "proxy certificate updated"},
		{false, true, "error", "certificate update error"},
		{true, false, "ok", "certificate validated"},
	} {
		restorer = service.MockProxyUpdateCert(func(validateOnly bool, payload []byte, certPath, keyPath string) error {
			if tc.fail {
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
		s := session.New(opt.Spool, 0, true, nil, config.SessionInspectorsConfig{})
		defer s.Discard()

		msg := messages.NewFetchCtl("update-cert", "", tc.dryRun, nil)
		t.ch <- msg
		res := <-msg.Rch

		c.Assert(res.Status, Equals, tc.result)
		c.Assert(res.Message, Equals, tc.message)

		err = svc.Stop()
		c.Assert(err, IsNil)
	}
}

func (t *serviceSuite) TestFetchctlCreateSession(c *C) {
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
		globalPerm bool
		payload    string
		sid        string
		token      string
		timeout    time.Duration
		permissive bool
	}{
		{true, `{"session-id":"x", "token":"y", "timeout":0, "mode":"strict", "inspectors-configuration":""}`, "x", "y", time.Duration(0), false},
		{false, `{"session-id":"x", "token":"y", "timeout":60, "mode":"strict", "inspectors-configuration":""}`, "x", "y", time.Duration(1 * time.Minute), false},
		{true, `{"session-id":"x", "token":"y", "timeout":0, "mode":"permissive", "inspectors-configuration":""}`, "x", "y", time.Duration(0), true},
		{false, `{"session-id":"x", "token":"y", "timeout":0, "mode":"permissive", "inspectors-configuration":""}`, "x", "y", time.Duration(0), false},
	} {
		var ss *session.Session
		restorer = service.MockSessionNewWithId(func(sessionId, token, spool string, timeout time.Duration, permissive bool, sec []secrets.Secret, inspectorsConfig config.SessionInspectorsConfig) *session.Session {
			c.Check(sessionId, Equals, tc.sid)
			c.Check(token, Equals, tc.token)
			c.Check(timeout, Equals, tc.timeout)
			c.Check(permissive, Equals, tc.permissive)

			ss = session.NewWithId(sessionId, token, spool, timeout, permissive, sec, inspectorsConfig)
			return ss
		})
		defer restorer()

		spool := filepath.Join(dir, "spool")
		opt := service.Options{
			ProxyPort:      1337,
			Spool:          spool,
			CertPath:       certPath,
			KeyPath:        keyPath,
			PermissiveMode: tc.globalPerm,
		}
		svc, err := service.New(&opt)
		c.Assert(err, IsNil)

		err = svc.Start()
		c.Assert(err, IsNil)

		msg := messages.NewFetchCtl("create-session", "", false, []byte(tc.payload))
		t.ch <- msg
		res := <-msg.Rch
		defer ss.Finish() // nolint:errcheck

		var policy string
		if tc.permissive {
			policy = "permissive"
		} else {
			policy = "strict"
		}

		c.Assert(res.Status, Equals, "ok")
		c.Assert(res.Message, Equals, fmt.Sprintf("session x:y created (%s)", policy))

		err = svc.Stop()
		c.Assert(err, IsNil)
	}
}

type loadConfigsOrDefaultTest struct {
	isSnap       bool   // Whether we're running from snap
	hasConfig    bool   // Whether the user has created a configuration file
	finalConfDir string // The directory configuration will be loaded from
}

var loadConfigsOrDefaultTests = []loadConfigsOrDefaultTest{{
	isSnap:       false,
	hasConfig:    false,
	finalConfDir: "/user/config",
}, {
	isSnap:       false,
	hasConfig:    true,
	finalConfDir: "/user/config",
}, {
	isSnap:       true,
	hasConfig:    false,
	finalConfDir: "/snap/fetch-service/x1/conf",
}, {
	isSnap:       true,
	hasConfig:    true,
	finalConfDir: "/user/config",
}}

func (t *serviceSuite) TestLoadHttpProxyRulesOrDefault(c *C) {
	var aclConfigDir string

	for _, tc := range loadConfigsOrDefaultTests {
		restorer := service.MockConfigLoadProxyHttpRules(func(cfgdir string) error {
			aclConfigDir = cfgdir
			if tc.hasConfig && cfgdir == "/user/config" {
				return nil
			}
			if tc.isSnap && cfgdir == "/snap/fetch-service/x1/conf" {
				return nil
			}
			return os.ErrNotExist
		})
		defer restorer()

		if tc.isSnap {
			os.Setenv("SNAP", "/snap/fetch-service/x1")
			defer os.Unsetenv("SNAP")

			os.Setenv("SNAP_NAME", "fetch-service")
			defer os.Unsetenv("SNAP_NAME")
		}

		aclConfigDir = ""
		err := service.LoadHttpProxyRulesOrDefault("/user/config")
		c.Assert(err, IsNil)
		c.Assert(aclConfigDir, Equals, tc.finalConfDir, Commentf("test case: %+v", tc))
	}
}

func (t *serviceSuite) TestInspectorsConfigOrDefault(c *C) {
	var inspConfigDir string

	for _, tc := range loadConfigsOrDefaultTests {
		restorer := service.MockConfigLoadInspectorsConfig(func(cfgdir string) error {
			inspConfigDir = cfgdir
			if tc.hasConfig && cfgdir == "/user/config" {
				return nil
			}
			if tc.isSnap && cfgdir == "/snap/fetch-service/x1/conf" {
				return nil
			}
			return os.ErrNotExist
		})
		defer restorer()

		if tc.isSnap {
			os.Setenv("SNAP", "/snap/fetch-service/x1")
			os.Setenv("SNAP_NAME", "fetch-service")
		}

		inspConfigDir = ""
		err := service.LoadInspectorsConfigOrDefault("/user/config")
		c.Assert(err, IsNil)
		c.Assert(inspConfigDir, Equals, tc.finalConfDir, Commentf("test case: %+v", tc))

		os.Unsetenv("SNAP")
		os.Unsetenv("SNAP_NAME")
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
