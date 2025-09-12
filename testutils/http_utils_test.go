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

package testutils_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/testutils"
)

type httpUtilsSuite struct {
}

func (t *httpUtilsSuite) SetUpTest(c *C) {
}

func (t *httpUtilsSuite) TearDownTest(c *C) {
}

var _ = Suite(&httpUtilsSuite{})

func (t *httpUtilsSuite) TestHTTPDownload(c *C) {
	tmp := c.MkDir()
	testFile := filepath.Join(tmp, "foo.txt")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a response
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello world"))
	}))
	defer server.Close()

	err := testutils.HTTPDownload(server.URL, testFile)
	c.Assert(err, IsNil)

	data, err := os.ReadFile(testFile)
	c.Assert(err, IsNil)
	c.Check(data, DeepEquals, []byte("hello world"))
}
