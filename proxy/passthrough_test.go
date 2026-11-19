package proxy

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/logger/testlogger"
	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
)

func TestPassthroughStreamsResponse(t *testing.T) {
	for _, permissive := range []bool{false, true} {
		t.Run(fmt.Sprintf("permissive=%t", permissive), func(t *testing.T) {
			allowLocalTestOrigin(t)
			testlogger.Init(logger.InfoLevel)
			proxyURL, spool := startTestProxy(t, func(messages.RequestInspection) error {
				t.Error("unexpected request inspection")
				return errors.New("unexpected request inspection")
			})
			s := session.NewWithPassthrough(spool, 0, permissive, true, nil, config.OverrideInspectorsConfig{})
			defer s.Discard()
			client := proxyTestClient(t, proxyURL, s)

			const path = "/one//two/../artifact%2Fname"
			const target = path + "?token=private-query"
			var requests atomic.Int32
			release := make(chan struct{}, 1)
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				body, err := io.ReadAll(r.Body)
				if err != nil || r.Method != http.MethodPost || r.RequestURI != target || string(body) != "payload" {
					t.Errorf("unexpected origin request: %s %s, body %q (%v)", r.Method, r.RequestURI, body, err)
				}
				if r.Header.Get(sessionIDHeader) != "" || r.Header.Get("Proxy-Authorization") != "" {
					t.Error("proxy credentials or session ID reached the origin")
				}
				w.Header().Set("X-Origin", "unchanged")
				_, _ = io.WriteString(w, "first")
				if err := http.NewResponseController(w).Flush(); err != nil {
					t.Error(err)
					return
				}
				<-release
				_, _ = io.WriteString(w, "last")
			}))
			defer origin.Close()
			defer close(release)

			post := func(want int) *http.Response {
				t.Helper()
				resp, err := client.Post(origin.URL+target, "text/plain", strings.NewReader("payload"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { resp.Body.Close() })
				if resp.StatusCode != want {
					t.Fatalf("status = %d, want %d", resp.StatusCode, want)
				}
				return resp
			}
			resp := post(http.StatusOK)
			first := make([]byte, len("first"))
			if _, err := io.ReadFull(resp.Body, first); err != nil || string(first) != "first" {
				t.Fatalf("first chunk = %q (%v)", first, err)
			}
			release <- struct{}{}
			last, err := io.ReadAll(resp.Body)
			if err != nil || string(last) != "last" || resp.Header.Get("X-Origin") != "unchanged" || len(s.Artifacts()) != 0 {
				t.Fatalf("unexpected response: %q (%v), headers %v, artifacts %d", last, err, resp.Header, len(s.Artifacts()))
			}
			if !testlogger.Contains(fmt.Sprintf(`passthrough request session=%q client="127.0.0.1" method="POST" url=%q`, s.ID, origin.URL+path)) || testlogger.Contains("private-query") {
				t.Error("audit record missing or query not redacted")
			}

			config.SetHTTPProxyConfig(config.HTTPProxyConfig{Policy: config.Deny})
			post(http.StatusForbidden)
			s.Revoke(s.Token)
			post(http.StatusProxyAuthRequired)
			if got := requests.Load(); got != 1 {
				t.Fatalf("origin received %d requests, want only the allowed request", got)
			}
		})
	}
}
