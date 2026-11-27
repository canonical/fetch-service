package proxy

import (
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/secrets"
	"github.com/canonical/fetch-service/service/config"
	"github.com/canonical/fetch-service/service/messages"
	"github.com/canonical/fetch-service/session"
	"github.com/canonical/fetch-service/testutils"
)

func startTestProxy(t *testing.T, inspect func(messages.RequestInspection) error) (*url.URL, string) {
	t.Helper()
	spool := t.TempDir()
	ch := make(chan any)
	t.Cleanup(func() { close(ch) })
	go func() {
		for msg := range ch {
			switch m := msg.(type) {
			case messages.ProxyAuth:
				m.Rch <- session.CheckAuth(m.ID, m.Pw, m.HostIP, m.ClientIP, m.Agent)
			case messages.RequestInspection:
				m.Rch <- inspect(m)
			case messages.ResponseInspection:
				t.Error("unexpected response inspection")
				m.Rch <- errors.New("unexpected response inspection")
			}
		}
	}()
	p, err := NewHTTPProxy(0, spool, testutils.ProxyCert, testutils.ProxyKey, ch)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(p.proxy)
	t.Cleanup(server.Close)
	proxyURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return proxyURL, spool
}

func allowLocalTestOrigin(t *testing.T) {
	t.Helper()
	t.Setenv("NO_PROXY", "127.0.0.1")
	cfg := config.GetHTTPProxyConfig()
	t.Cleanup(func() { config.SetHTTPProxyConfig(cfg) })
	config.SetHTTPProxyConfig(config.HTTPProxyConfig{Policy: config.Allow})
}

func proxyTestClient(t *testing.T, proxyURL *url.URL, s *session.Session) *http.Client {
	t.Helper()
	u := *proxyURL
	u.User = url.UserPassword(s.ID, s.Token)
	tr := &http.Transport{Proxy: http.ProxyURL(&u), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	t.Cleanup(tr.CloseIdleConnections)
	return &http.Client{Transport: tr, Timeout: 5 * time.Second}
}

func TestProxyPreservesRequestTarget(t *testing.T) {
	allowLocalTestOrigin(t)
	const target = "/one//two/../three%2Ffour?key=value"
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI != target {
			t.Errorf("origin request target = %q, want %q", r.RequestURI, target)
		}
		if user, _, _ := r.BasicAuth(); user != "exact-target" {
			t.Error("secret matching did not use the exact request target")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer origin.Close()

	proxyURL, spool := startTestProxy(t, func(m messages.RequestInspection) error {
		if got := m.A.CurrentDownload.URL; got != origin.URL+target {
			t.Errorf("inspection URL = %q, want %q", got, origin.URL+target)
		}
		return nil
	})
	s := session.New(spool, 0, false, []secrets.Secret{
		{Type: secrets.BasicAuthType, URL: glob.MustCompile(origin.URL + "/one/three/four*"), BasicCreds: "normalized-target:password"},
		{Type: secrets.BasicAuthType, URL: glob.MustCompile(origin.URL + "/one//two/../three%2Ffour*"), BasicCreds: "exact-target:password"},
	}, config.OverrideInspectorsConfig{})
	defer s.Discard()
	client := proxyTestClient(t, proxyURL, s)
	resp, err := client.Get(origin.URL + target)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestProxyDeniesBeforeSend(t *testing.T) {
	allowLocalTestOrigin(t)
	config.SetHTTPProxyConfig(config.HTTPProxyConfig{Policy: config.Deny})
	var requests atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer origin.Close()

	proxyURL, spool := startTestProxy(t, func(messages.RequestInspection) error { return nil })
	s := session.New(spool, 0, false, nil, config.OverrideInspectorsConfig{})
	defer s.Discard()
	resp, err := proxyTestClient(t, proxyURL, s).Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusForbidden || string(body) != "Access denied" {
		t.Fatalf("response = %d %q (%v), want 403 Access denied", resp.StatusCode, body, err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("origin received %d denied requests, want none", got)
	}
}

func TestProxyTunnelDoesNotAdoptReplacementSession(t *testing.T) {
	proxyURL, spool := startTestProxy(t, func(messages.RequestInspection) error {
		return errors.New("test inspection denial")
	})
	s := session.NewWithID("reusable-id", "old-token", spool, 0, false, nil, config.OverrideInspectorsConfig{})
	t.Cleanup(func() {
		if s.IsActive() {
			s.Discard()
		}
	})
	client := proxyTestClient(t, proxyURL, s)
	get := func(want string) {
		t.Helper()
		resp, err := client.Get("https://example.invalid/")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != http.StatusForbidden || string(body) != want {
			t.Fatalf("response = %d %q (%v), want 403 %q", resp.StatusCode, body, err, want)
		}
	}
	get("download authorization denied: test inspection denial")
	s.Discard()
	replacement := session.NewWithID(s.ID, "new-token", spool, 0, true, nil, config.OverrideInspectorsConfig{})
	defer replacement.Discard()
	// A new CONNECT would fail authentication; the existing tunnel must be
	// rejected before it reaches the replacement session's inspectors.
	get("Session is no longer active")
}
