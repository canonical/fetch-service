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

package proxy_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/proxy"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
	"github.com/canonical/fetch-service/utils"
)

func Test(t *testing.T) { TestingT(t) }

type proxySuite struct{}

func (t *proxySuite) SetUpTest(c *C) {
	testlogger.Init(logger.InfoLevel)
}

var _ = Suite(&proxySuite{})

var (
	certData = []byte(
		`-----BEGIN CERTIFICATE-----
MIIFEzCCAvugAwIBAgIUDNMle3u8YpnJyST0XyHywxcv51YwDQYJKoZIhvcNAQEL
BQAwGTEXMBUGA1UEAwwOcm9vdEBsb2NhbGhvc3QwHhcNMjQwNjIwMTkyNTI0WhcN
NDQwNjE1MTkyNTI0WjAZMRcwFQYDVQQDDA5yb290QGxvY2FsaG9zdDCCAiIwDQYJ
KoZIhvcNAQEBBQADggIPADCCAgoCggIBAKKTVtzXreoF4BtMckjJMFwANfsAOrM4
NgQtVYWuuS+ZFPALa5xLlRkKN66YHbjL6oUZgFM9NzNsx82cpc5U9Nl4hSGJaGvg
W7aGkfdFGOki+m5W1ZJLW5GhTXLXILQpNvdJbltxZHIZ6J1VvZx1WddPhS91pjmc
eY2EPl5L6VNriho6JAzPmzqNRBTguV6LjCEjwdtm1dPNgsUCKQvhSp/uFtQ5wazo
ImruSyHImZ1xANyhgAoMxsvaSzC6tL02ioxyBjCDCohFrSNgjBmQ7vUXddqH+BJC
21rzvYXSAOSJ5oH2QnGBIIhSz4AuudQl/jBL+pcs0eveFEenvay1YfvvVY/yPzqL
kE80OW1ijDEq1Ak16hN1FBczwSdkf0yZMZOpjnyHuwP1NeQFvX5BmG+drHBNm7Af
4EfrhiB/QGCZPw7ec8fySjCYwlQjORlJk9wcfFWEeYzPamRF1LgwBeXtHahZyy9c
5z01LkUJLdeMGX2l6MWOwm95emsMgVvb4n1vnRqMv4nabC1mn+UoJlnspS8fwWAa
SDau6KD9NF3bptgFDhVB3nRPKaaHkPSAyAZ3nbgGzyoPxnGx/HzzfNc1AP0i0mK8
NQ40TTYmD53kQU9F62GlFxkDRuf9jk8uBCORDGwJdBjv4j87+miitr0gOinRfzRK
Fx37Jppcxua5AgMBAAGjUzBRMB0GA1UdDgQWBBT14F3kFUUBuJcyD9CcdSDiuwdU
5TAfBgNVHSMEGDAWgBT14F3kFUUBuJcyD9CcdSDiuwdU5TAPBgNVHRMBAf8EBTAD
AQH/MA0GCSqGSIb3DQEBCwUAA4ICAQATQzWTFg7MdQL1bDAhytiQFh06ujbWEZdV
0c5I6q0xAmXDo+alFHPSBfnOIk6QXgclV0NKmA4visXnNf9MFeZL5Pxbc0Qo3p9e
iutw7XXTrrOg/ZED8dX7VoVVlvOqCcVhyGhZWUudTGF5EoPNv/UK3WeTTSPXHlsf
a84CRiEyI4t/K3kHjWkOyhm2fxcHtdyj6/9mVnHtW0UyjGCaYk/tfjL/eneT/kgm
UXj40a+84+3plfsK4kE91Ziw+u99KHK4FzVOm5EN0wAdEzfapZd8zq6N/nn2WRA/
KEwgS3bXauPuv4XubEnfL7MbF8AzgKmyg/j3//+TRsgFe6JyMuOoMtDfsgliEy71
A7Ci/UDnMHvOrWbmtAjSpXZat8I6shabyuJeX1oM5G67Y7TPQFpuGmNi8p3eUFkx
qbKGkvdehhIduPNc5wAcFqMNmBYhon2U+GDxFvuhh55LnUWaq3r0+SN4ykHVK9lH
h89VPO73YN89SXeyNV+0SemS+C5m4S/XAkox4yQDWY/8O36sGRArlHPdU8Oyjks/
S97wdpffrGO3KQbYQm+I0XZQWDRZZohbusy+yfoA7xOHqHeozfHXSzSVZa8egQsz
DlLEQlTlQOMRCnnGnMznpOcYmbir3STCB482LW2QEFihb+C0Wa+uuVroNKvkBiKc
GD14ie2tJw==
-----END CERTIFICATE-----`)

	keyData = []byte(
		`-----BEGIN PRIVATE KEY-----
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
Lg6VXCYkKYD1Gdm1Cd5oxEPBJnsg
-----END PRIVATE KEY-----`)
)

func (t *proxySuite) TestSetProxyCertificate(c *C) {
	err := proxy.SetProxyCertificate(certData, keyData)
	c.Assert(err, IsNil)
}

// Test file transfer using the proxy.
func (t *proxySuite) TestProxyDownload(c *C) {
	// start the fetch service proxy
	ch := make(chan interface{}, 1)
	spool := c.MkDir()
	p, err := proxy.NewHttpProxy(5566, spool, certData, keyData, ch)
	c.Assert(err, IsNil)

	err = p.Start()
	c.Assert(err, IsNil)
	defer func() {
		err := p.Stop()
		c.Assert(err, IsNil)
	}()

	time.Sleep(1 * time.Second)

	// create a new session
	s := session.New(spool, true)
	defer s.Discard()

	// download a test file
	proxyURL := url.URL{
		Scheme: "http",
		User:   url.UserPassword(s.Id, s.Token),
		Host:   "localhost:5566",
	}

	url, err := url.Parse("https://launchpadlibrarian.net/592566337/hello_2.10-2ubuntu4_amd64.deb")
	c.Assert(err, IsNil)

	transport := &http.Transport{
		Proxy:           http.ProxyURL(&proxyURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	go func() {
		req, err := http.NewRequest("GET", url.String(), nil)
		c.Assert(err, IsNil)

		resp, err := client.Do(req)
		c.Assert(err, IsNil)
		c.Assert(resp.StatusCode, Equals, 200)
	}()

	// authorize download
	msg := <-ch
	auth := msg.(messages.ProxyAuth)
	c.Assert(auth.Id, Equals, s.Id)
	c.Assert(auth.Pw, Equals, s.Token)
	auth.Rch <- true

	// run request inspectors
	msg = <-ch
	v := msg.(messages.RequestInspection)
	v.Rch <- nil // no errors

	// artefact downloaded
	msg = <-ch
	u := msg.(messages.ResponseInspection)

	dest := filepath.Join(u.A.AssetDir, fmt.Sprintf("%s.data", u.A.Metadata.Sha256))
	err = os.MkdirAll(filepath.Dir(dest), 0755)
	c.Assert(err, IsNil)

	err = utils.MoveFile(u.A.Tempfile, dest)
	c.Assert(err, IsNil)
	os.Remove(u.A.Tempfile)

	// check downloaded file information
	c.Assert(v.A.MetadataVersion, Equals, "0.1")
	c.Assert(u.A.Metadata.Sha1.String(), Equals, "d8c1f9634007b54c1e9aa3ba3b51395b643933c3")
	c.Assert(u.A.Metadata.Sha256.String(), Equals, "750335248ccc68d07397e2b843d94fd1a164ddeca23892ca8398b5d528cd89eb")
	c.Assert(u.A.Metadata.Size, Equals, int64(26600))

	dl := u.A.CurrentDownload
	c.Assert(dl.StatusCode, Equals, 200)
	c.Assert(dl.URL, Equals, "https://launchpadlibrarian.net:443/592566337/hello_2.10-2ubuntu4_amd64.deb")
	c.Assert(dl.Method, Equals, "GET")
	c.Assert(dl.ContentType, Equals, "application/x-debian-package")
	c.Assert(dl.UserAgent, Equals, "Go-http-client/1.1")
	c.Assert(dl.RequestHeader, DeepEquals, map[string][]string{
		"Accept-Encoding": []string{"gzip"},
		"User-Agent":      []string{"Go-http-client/1.1"},
	})
	c.Assert(dl.ResponseHeader["Content-Length"], DeepEquals, []string{"26600"})
	c.Assert(dl.ResponseHeader["Content-Type"], DeepEquals, []string{"application/x-debian-package"})

	u.Rch <- nil // no errors
}

func (t *proxySuite) TestCopyHeader(c *C) {
	for _, tc := range []struct {
		key string
		val []string
	}{
		{"key", []string{}},
		{"key", []string{"a", "b", "c"}},
	} {
		data := map[string][]string{tc.key: tc.val}
		newData := proxy.CopyHeader(data)
		delete(data, tc.key)
		c.Assert(data[tc.key], IsNil)
		c.Assert(newData, Not(Equals), data)
		c.Assert(newData[tc.key], DeepEquals, tc.val)
		c.Assert(newData[tc.key], Not(Equals), tc.val)
	}
}
