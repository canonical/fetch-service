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

package seclog

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/utils"
	"github.com/canonical/fetch-service/version"
)

const LevelCritical = slog.Level(12)

type EventData struct {
	User       string
	Identity   string
	HostIP     string
	ClientIP   string
	Agent      string
	Reason     string
	RequestURL string
}

var (
	secl      *slog.Logger = slog.New(slog.DiscardHandler)
	logWriter *logger.LogWriter
)

func Init(logfile string) error {
	logWriter = &logger.LogWriter{Path: logfile}
	if err := logWriter.Reopen(); err != nil {
		return err
	}

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo, // Set the minimum level to log
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				// Rename default "time" to "datetime" for OWASP
				a.Key = "datetime"

			case slog.MessageKey:
				// Use "msg" as "description"
				a.Key = "description"
				// Add log level CRITICAL
			case slog.LevelKey:
				level := a.Value.Any().(slog.Level)
				if level == LevelCritical {
					a.Value = slog.StringValue("CRITICAL")
				}
			}

			return a
		},
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("cannot obtain host name: %w", err)
	}

	secl = slog.New(slog.NewJSONHandler(logWriter, opts)).With(
		"hostname", hostname,
		"type", "security",
		"appid", fmt.Sprintf("fetch-service:%s", version.Version))

	return nil
}

func Close() error {
	if logWriter != nil {
		if err := logWriter.Close(); err != nil {
			return fmt.Errorf("cannot close security log file: %w", err)
		}
	}
	return nil
}

func Reopen() error {
	if logWriter != nil {
		logger.Warningf("reopening security log file %s", logWriter.Path)

		if err := logWriter.Reopen(); err != nil {
			return fmt.Errorf("cannot reopen security log file: %w", err)
		}
	}
	return nil
}

// AuthnLoginSuccess generates an authn_login_success log event.
//
// Successful logins are logged each time a user or service account successfully
// authenticates and gains access to the system, helping to establish baseline
// authentication patterns and enabling to identify deviations, anomalies or
// unexpected login behaviors (such as unusual locations or atypical user-agent
// data) that could indicate potential security risks.
func AuthnLoginSuccess(ev *EventData) {
	secl.Info(fmt.Sprintf("User %s authentication successful", ev.User),
		"event", fmt.Sprintf("authn_login_success:%s", ev.User),
		"host_ip", ev.HostIP,
		"client_ip", ev.ClientIP,
		"user_agent", ev.Agent,
		"identity", ev.Identity)
}

// AuthnLoginFail generates an authn_login_fail log event.
//
// Failed login events are logged each time a user or service account fails to
// authenticate to the system. These logs enable the configuration of frequency-
// and volume-based detection rules, triggering alerts for suspicious patterns
// of failed authentication attempts, such as password spraying, probing, or
// brute-force attacks.
func AuthnLoginFail(ev *EventData) {
	secl.Warn(fmt.Sprintf("User %s authentication fail", ev.User),
		"event", fmt.Sprintf("authn_login_fail:%s", ev.User),
		"host_ip", ev.HostIP,
		"client_ip", ev.ClientIP,
		"user_agent", ev.Agent,
		"request_url", ev.RequestURL,
		"reason", ev.Reason)
}

// AuthnTokenCreated generates an authn_token_created log event.
//
// Event logs related to the lifecycle of authentication tokens (including token
// creation, revocation, attempted reuse, and deletion) play a critical role in
// monitoring token-based authentication systems and detecting suspicious
// activities related to token abuse, such as replay attacks, where stolen tokens
// are used to impersonate legitimate users, and attackers trying to quickly
// compromise and discard tokens, masking their activities.
func AuthnTokenCreated(ev *EventData) {
	secl.Info(fmt.Sprintf("Access token issued for session %s", ev.User),
		"event", fmt.Sprintf("authn_token_created:%s", ev.User),
		"host_ip", ev.HostIP,
		"client_ip", ev.ClientIP,
		"user_agent", ev.Agent,
		"identity", ev.Identity,
		"token_type", "FETCH_SERVICE_SESSION")
}

// AuthnTokenRevoked generates an authn_token_revoked log event.
func AuthnTokenRevoked(ev *EventData) {
	secl.Info(fmt.Sprintf("Access token revoked for session %s", ev.User),
		"event", fmt.Sprintf("authn_token_revoked:%s", ev.User),
		"host_ip", ev.HostIP,
		"client_ip", ev.ClientIP,
		"user_agent", ev.Agent,
		"identity", ev.Identity,
		"reason", ev.Reason)
}

// AuthnTokenReuse generates an authn_token_reuse log event.
func AuthnTokenReuse(ev *EventData) {
	secl.Log(context.Background(), LevelCritical,
		fmt.Sprintf("Attempt to use a revoked token for session %s", ev.User),
		"event", fmt.Sprintf("authn_token_reuse:%s", ev.User),
		"host_ip", ev.HostIP,
		"client_ip", ev.ClientIP,
		"user_agent", ev.Agent,
		"identity", ev.Identity,
		"request_url", ev.RequestURL)
}

// AuthnTokenDelete generates an authn_token_delete log event.
func AuthnTokenDelete(ev *EventData) {
	secl.Info(fmt.Sprintf("Access token deleted for session %s", ev.User),
		"event", fmt.Sprintf("authn_token_delete:%s", ev.User),
		"host_ip", ev.HostIP,
		"client_ip", ev.ClientIP,
		"user_agent", ev.Agent,
		"identity", ev.Identity,
		"reason", ev.Reason)
}

// AuthzAdmin generates an authz_admin log event.
//
// Administrative activity events log actions performed by users or entities with
// elevated privileges. These events need to be carefully monitored due to the level
// of access they provide, which, if misused, could lead to significant security
// vulnerabilities or compliance violations.
func AuthzAdmin(ev *EventData, admEvent string) {
	secl.Warn(fmt.Sprintf("Administrative command %s", admEvent),
		"event", fmt.Sprintf("authz_admin:%s, %s", ev.User, admEvent),
		"host_ip", ev.HostIP,
		"client_ip", ev.ClientIP,
		"user_agent", ev.Agent,
		"identity", ev.Identity)
}

// SysStart generates a sys_start log event.
//
// System start events help to identify possible unauthorized changes or failures
// in the boot process, which may be indicative of tampering or malfunction, and
// also supports troubleshooting by identifying the boot time for further
// diagnostic actions.
func SysStart(ev *EventData, policyMode string) {
	secl.Info("Fetch Service started successfully",
		"event", "sys_start",
		"version", version.Version,
		"runtime_env", utils.RuntimeEnv(),
		"policy_mode", policyMode)
}

// SysShutdown generates a sys_shutdown log event.
//
// System shutdown events capture when a system or service is stopped, allowing to
// track scheduled maintenance, failures, or unplanned shutdowns, which can indicate
// issues, misconfigurations, and also potential security breaches or availability
// attacks.
func SysShutdown(ev *EventData) {
	secl.Info("Fetch Service shutting down",
		"event", "sys_shutdown",
		"reason", ev.Reason,
		"runtime_env", utils.RuntimeEnv())
}

// SysCrash generates a sys_crash log event.
//
// System crash events capture the circumstances that led to a failure and is
// essential for incident response teams to quickly identify root causes (post-
// mortem analysis), fix any issues, and restore system functionality.
func SysCrash(ev *EventData, crashEvent, crashDesc string, stackTrace []byte) {
	secl.Log(context.Background(), LevelCritical,
		crashDesc,
		"event", fmt.Sprintf("sys_crash:%s", crashEvent),
		"stack_trace", string(stackTrace),
		"go_version", runtime.Version(),
		"runtime_env", utils.RuntimeEnv())

}
