// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2026 Canonical Ltd.
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

package seclog_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/seclog"
	"github.com/canonical/fetch-service/version"
)

func Test(t *testing.T) { TestingT(t) }

type seclogSuite struct {
	event *seclog.EventData
}

var _ = Suite(&seclogSuite{
	&seclog.EventData{
		User:       "joebob1",
		HostIP:     "1.1.1.1",
		ClientIP:   "2.2.2.2",
		Agent:      "TestAgent/007",
		Identity:   "uid",
		RequestURL: "/test/seclog",
		Reason:     "Something happened",
	},
})

func (t *seclogSuite) TestAuthnLoginSuccess(c *C) {
	logFile := filepath.Join(c.MkDir(), "test.log")

	err := seclog.Init(logFile)
	c.Assert(err, IsNil)

	seclog.AuthnLoginSuccess(t.event)

	err = seclog.Close()
	c.Assert(err, IsNil)

	logs := parseLog(logFile)
	c.Assert(logs, HasLen, 1)
	checkLog(c, logs[0], map[string]any{
		"level":       "INFO",
		"event":       "authn_login_success:joebob1",
		"host_ip":     "1.1.1.1",
		"client_ip":   "2.2.2.2",
		"user_agent":  "TestAgent/007",
		"description": "User joebob1 authentication successful",
		"identity":    "uid",
	})
}

func (t *seclogSuite) TestAuthnLoginFail(c *C) {
	logFile := filepath.Join(c.MkDir(), "test.log")

	err := seclog.Init(logFile)
	c.Assert(err, IsNil)

	seclog.AuthnLoginFail(t.event)

	err = seclog.Close()
	c.Assert(err, IsNil)

	logs := parseLog(logFile)
	c.Assert(logs, HasLen, 1)
	checkLog(c, logs[0], map[string]any{
		"level":       "WARN",
		"event":       "authn_login_fail:joebob1",
		"host_ip":     "1.1.1.1",
		"client_ip":   "2.2.2.2",
		"user_agent":  "TestAgent/007",
		"description": "User joebob1 authentication failed",
		"request_url": "/test/seclog",
		"reason":      "Something happened",
	})
}

func (t *seclogSuite) TestAuthnTokenCreated(c *C) {
	logFile := filepath.Join(c.MkDir(), "test.log")

	err := seclog.Init(logFile)
	c.Assert(err, IsNil)

	seclog.AuthnTokenCreated(t.event)

	err = seclog.Close()
	c.Assert(err, IsNil)

	logs := parseLog(logFile)
	c.Assert(logs, HasLen, 1)
	checkLog(c, logs[0], map[string]any{
		"level":       "INFO",
		"event":       "authn_token_created:joebob1",
		"host_ip":     "1.1.1.1",
		"client_ip":   "2.2.2.2",
		"user_agent":  "TestAgent/007",
		"description": "Access token issued for session joebob1",
		"identity":    "uid",
		"token_type":  "FETCH_SERVICE_SESSION",
	})
}

func (t *seclogSuite) TestAuthnTokenRevoked(c *C) {
	logFile := filepath.Join(c.MkDir(), "test.log")

	err := seclog.Init(logFile)
	c.Assert(err, IsNil)

	seclog.AuthnTokenRevoked(t.event)

	err = seclog.Close()
	c.Assert(err, IsNil)

	logs := parseLog(logFile)
	c.Assert(logs, HasLen, 1)
	checkLog(c, logs[0], map[string]any{
		"level":       "INFO",
		"event":       "authn_token_revoked:joebob1",
		"host_ip":     "1.1.1.1",
		"client_ip":   "2.2.2.2",
		"user_agent":  "TestAgent/007",
		"description": "Access token revoked for session joebob1",
		"identity":    "uid",
		"reason":      "Something happened",
	})
}

func (t *seclogSuite) TestAuthnTokenReuse(c *C) {
	logFile := filepath.Join(c.MkDir(), "test.log")

	err := seclog.Init(logFile)
	c.Assert(err, IsNil)

	seclog.AuthnTokenReuse(t.event)

	err = seclog.Close()
	c.Assert(err, IsNil)

	logs := parseLog(logFile)
	c.Assert(logs, HasLen, 1)
	checkLog(c, logs[0], map[string]any{
		"level":       "CRITICAL",
		"event":       "authn_token_reuse:joebob1",
		"host_ip":     "1.1.1.1",
		"client_ip":   "2.2.2.2",
		"user_agent":  "TestAgent/007",
		"description": "Attempt to use a revoked token for session joebob1",
		"identity":    "uid",
		"request_url": "/test/seclog",
	})
}

func (t *seclogSuite) TestAuthnTokenDelete(c *C) {
	logFile := filepath.Join(c.MkDir(), "test.log")

	err := seclog.Init(logFile)
	c.Assert(err, IsNil)

	seclog.AuthnTokenDelete(t.event)

	err = seclog.Close()
	c.Assert(err, IsNil)

	logs := parseLog(logFile)
	c.Assert(logs, HasLen, 1)
	checkLog(c, logs[0], map[string]any{
		"level":       "INFO",
		"event":       "authn_token_delete:joebob1",
		"host_ip":     "1.1.1.1",
		"client_ip":   "2.2.2.2",
		"user_agent":  "TestAgent/007",
		"description": "Access token deleted for session joebob1",
		"identity":    "uid",
		"reason":      "Something happened",
	})
}

func (t *seclogSuite) TestAuthzAdmin(c *C) {
	logFile := filepath.Join(c.MkDir(), "test.log")

	err := seclog.Init(logFile)
	c.Assert(err, IsNil)

	seclog.AuthzAdmin(t.event, "admin_cmd")

	err = seclog.Close()
	c.Assert(err, IsNil)

	logs := parseLog(logFile)
	c.Assert(logs, HasLen, 1)
	checkLog(c, logs[0], map[string]any{
		"level":       "WARN",
		"event":       "authz_admin:joebob1, admin_cmd",
		"host_ip":     "1.1.1.1",
		"client_ip":   "2.2.2.2",
		"user_agent":  "TestAgent/007",
		"description": "Administrative command admin_cmd",
		"identity":    "uid",
	})
}

func (t *seclogSuite) TestSysStart(c *C) {
	logFile := filepath.Join(c.MkDir(), "test.log")

	err := seclog.Init(logFile)
	c.Assert(err, IsNil)

	seclog.SysStart(t.event, "strict-only")

	err = seclog.Close()
	c.Assert(err, IsNil)

	logs := parseLog(logFile)
	c.Assert(logs, HasLen, 1)
	checkLog(c, logs[0], map[string]any{
		"level":       "INFO",
		"event":       "sys_start",
		"version":     version.Version,
		"description": "Fetch Service started successfully",
		"policy_mode": "strict-only",
		"runtime_env": "unknown",
	})
}

func (t *seclogSuite) TestSysShutdown(c *C) {
	logFile := filepath.Join(c.MkDir(), "test.log")

	err := seclog.Init(logFile)
	c.Assert(err, IsNil)

	seclog.SysShutdown(t.event)

	err = seclog.Close()
	c.Assert(err, IsNil)

	logs := parseLog(logFile)
	c.Assert(logs, HasLen, 1)
	checkLog(c, logs[0], map[string]any{
		"level":       "INFO",
		"event":       "sys_shutdown",
		"description": "Fetch Service shutting down",
		"reason":      "Something happened",
		"runtime_env": "unknown",
	})
}

func (t *seclogSuite) TestSysCrash(c *C) {
	logFile := filepath.Join(c.MkDir(), "test.log")

	err := seclog.Init(logFile)
	c.Assert(err, IsNil)

	seclog.SysCrash(t.event, "nil_pointer_dereference", "Runtime panic", []byte("stack trace"))

	err = seclog.Close()

	c.Assert(err, IsNil)
	logs := parseLog(logFile)
	c.Assert(logs, HasLen, 1)
	checkLog(c, logs[0], map[string]any{
		"level":       "CRITICAL",
		"event":       "sys_crash:nil_pointer_dereference",
		"description": "Runtime panic",
		"go_version":  runtime.Version(),
		"stack_trace": "stack trace",
		"runtime_env": "unknown",
	})
}

func (t *seclogSuite) TestReopen(c *C) {
	logDir := c.MkDir()
	logFile := filepath.Join(logDir, "test.log")

	err := seclog.Init(logFile)
	c.Assert(err, IsNil)

	seclog.AuthnLoginFail(&seclog.EventData{Reason: "Test message 1"})
	seclog.Reopen()
	seclog.AuthnLoginFail(&seclog.EventData{Reason: "Test message 2"})

	oldLogFile := filepath.Join(logDir, "oldfile.log")
	err = os.Rename(logFile, oldLogFile)
	c.Assert(err, IsNil)
	seclog.Reopen()
	seclog.AuthnLoginFail(&seclog.EventData{Reason: "Test message 3"})

	// Read log file
	buf, err := os.ReadFile(logFile)
	c.Assert(err, IsNil)

	contents := string(buf)
	lines := strings.Split(contents, "\n")
	c.Assert(strings.Contains(lines[0], `"reason":"Test message 3"`), Equals, true)

	// Read renamed log file
	buf, err = os.ReadFile(oldLogFile)
	c.Assert(err, IsNil)

	contents = string(buf)
	lines = strings.Split(contents, "\n")
	c.Assert(strings.Contains(lines[0], `"reason":"Test message 1"`), Equals, true)
	c.Assert(strings.Contains(lines[1], `"reason":"Test message 2"`), Equals, true)
}

// parseLog parse a security log file into a slice of maps.
func parseLog(filename string) []map[string]any {
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var results []map[string]any
	decoder := json.NewDecoder(file)

	// Decode each JSON object in the file
	for decoder.More() {
		var m map[string]any
		if err := decoder.Decode(&m); err != nil {
			panic(err)
		}
		results = append(results, m)
	}

	return results
}

func checkLog(c *C, log map[string]any, expected map[string]any) {
	// Check appid
	c.Check(log["appid"], Equals, "fetch-service:"+version.Version)

	// Check type
	c.Check(log["type"], Equals, "security")

	// Check hostname
	hostname, err := os.Hostname()
	c.Check(log["hostname"], Equals, hostname)
	c.Assert(err, IsNil)

	// Check datetime
	datetime, err := time.Parse(time.RFC3339, log["datetime"].(string))
	c.Assert(err, IsNil)
	c.Check(time.Since(datetime) < time.Duration(2*time.Second), Equals, true)

	cleanLog := copyLogExcept(log, "appid", "type", "hostname", "datetime")
	c.Assert(cleanLog, DeepEquals, expected)
}

func copyLogExcept(log map[string]any, exclude ...string) map[string]any {
	skip := make(map[string]struct{})
	for _, k := range exclude {
		skip[k] = struct{}{}
	}

	res := make(map[string]any)
	for k, v := range log {
		if _, ok := skip[k]; !ok {
			res[k] = v
		}
	}
	return res
}
