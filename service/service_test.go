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

package service_test

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service"
)

func Test(t *testing.T) { TestingT(t) }

type serviceSuite struct {
	port int
}

func (t *serviceSuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&serviceSuite{})

// Check if the proxy is created with the correct port number.
func (s *serviceSuite) TestProxyPort(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, ch chan interface{}) *proxy.HttpProxy {
		s.port = port
		return &proxy.HttpProxy{}
	})
	defer restorer()

	opt := service.Options{ProxyPort: 1337}

	svc := service.New(&opt)
	c.Assert(svc, FitsTypeOf, &service.Service{})
	c.Assert(s.port, Equals, 1337)
}

func (s *serviceSuite) TestServiceEntombment(c *C) {
	restorer := service.MockNewHttpProxy(func(port int, spool string, ch chan interface{}) *proxy.HttpProxy {
		s.port = port
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
