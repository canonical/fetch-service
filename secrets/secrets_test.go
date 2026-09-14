package secrets_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/secrets"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type secretSuite struct {
	sl logger.Logger
}

var _ = Suite(&secretSuite{logger.NewSessionLogger("test")})

func (t *secretSuite) TestValidateSecrets(c *C) {
	for _, tc := range []struct {
		sec []secrets.Secret
		err error
	}{
		// No secrets
		{nil, nil},
		// Good basic-auth secret
		{[]secrets.Secret{{Type: secrets.BasicAuthType, URL: glob.MustCompile("www.example.com"), BasicCreds: "user:passwd"}}, nil},
		// Good macaroon secret
		{[]secrets.Secret{{Type: secrets.MacaroonType, URL: glob.MustCompile("www.example.com"), MacaroonCreds: "deadbeef"}}, nil},
		// Good keystone secret
		{[]secrets.Secret{{Type: secrets.KeystoneV3Type, URL: glob.MustCompile("www.example.com"), KeystoneV3Creds: "user:passwd"}}, nil},
		// Missing type
		{[]secrets.Secret{{}}, secrets.ErrMissingSecretType},
		// Invalid type
		{[]secrets.Secret{{Type: "invalid-type"}}, secrets.ErrInvalidSecretType},
		// Missing url
		{[]secrets.Secret{{Type: secrets.BasicAuthType}}, secrets.ErrMissingSecretURL},
	} {
		err := secrets.ValidateSecrets(tc.sec)
		c.Assert(err, Equals, tc.err)
	}
}

func (t *secretSuite) TestSecretsUnmarshalJSON(c *C) {
	type testGlob struct {
		Secrets []secrets.Secret `json:"secrets"`
	}

	data := []byte(`{
      "secrets": [
        {
          "type": "basic-auth",
          "url": "https://github.com:443/canonical/fetch-service.git/**",
          "basic-credentials": "user:passwd"
        },
        {
          "type": "macaroon",
          "url": "https://www.example.com/**",
          "macaroon-credentials": "deadbeef"
        },
        {
          "type": "keystone-v3",
	  "url": "https://www.example.com:5000/v3/auth/tokens",
	  "keystone-v3-credentials": "user:password"
        }
      ]
    }`)

	var j testGlob
	err := json.Unmarshal(data, &j)
	c.Assert(err, IsNil)

	c.Assert(len(j.Secrets), Equals, 3)

	c.Assert(j.Secrets[0].Type, Equals, secrets.BasicAuthType)
	c.Assert(j.Secrets[0].URL, DeepEquals, glob.MustCompile("https://github.com:443/canonical/fetch-service.git/**"))
	c.Assert(j.Secrets[0].BasicCreds, Equals, "user:passwd")

	c.Assert(j.Secrets[1].Type, Equals, secrets.MacaroonType)
	c.Assert(j.Secrets[1].URL, DeepEquals, glob.MustCompile("https://www.example.com/**"))
	c.Assert(j.Secrets[1].MacaroonCreds, Equals, "deadbeef")

	c.Assert(j.Secrets[2].Type, Equals, secrets.KeystoneV3Type)
	c.Assert(j.Secrets[2].URL, DeepEquals, glob.MustCompile("https://www.example.com:5000/v3/auth/tokens"))
	c.Assert(j.Secrets[2].KeystoneV3Creds, Equals, "user:password")
}

func (t *secretSuite) TestInjectHeaderSecrets(c *C) {
	sec := []secrets.Secret{
		{Type: secrets.BasicAuthType, URL: glob.MustCompile("https://github.com:443/canonical/fetch-service.git/**"), BasicCreds: "user:passwd"},
		{Type: secrets.MacaroonType, URL: glob.MustCompile("https://www.my-domain.com/**"), MacaroonCreds: "deadbeef"},
	}

	for _, tc := range []struct {
		url      string
		injected bool
		header   string
	}{
		{"www.example.com", false, ""},
		{"https://github.com:443/canonical/different-repo.git/", false, ""},
		{"https://github.com:443/canonical/fetch-service.git/", true, "Basic dXNlcjpwYXNzd2Q="},
		{"https://www.my-domain.com/endpoint/", true, "macaroon deadbeef"},
	} {
		req, err := http.NewRequest("GET", tc.url, nil)
		c.Assert(err, IsNil)

		injected, err := secrets.InjectSecrets(sec, tc.url, req, t.sl)
		c.Assert(err, IsNil)
		c.Assert(injected, Equals, tc.injected)
		if injected {
			header := req.Header.Get("Authorization")
			c.Assert(header, Equals, tc.header)
		}
	}
}

func (t *secretSuite) TestInjectBodySecrets(c *C) {
	sec := []secrets.Secret{
		{Type: secrets.KeystoneV3Type, URL: glob.MustCompile("https://my-domain.com:5000/v3/auth/tokens"), KeystoneV3Creds: "new-user:new-pass"},
	}

	body := []byte(`{
		"auth": {
			"identity": {
				"methods": ["password"],
				"password": { "user": { "name": "old-name", "password": "old-pass", "domain": {"name": "my-domain"} } }
			},
			"scope": { "project": { "name": "my-project", "domain": {"name": "other-domain"} } },
			"extra-field": "extra-content"
		}
	}`)

	req, err := http.NewRequest("GET", "https://my-domain.com:5000/v3/auth/tokens", bytes.NewReader(body))
	c.Assert(err, IsNil)

	injected, err := secrets.InjectSecrets(sec, "https://my-domain.com:5000/v3/auth/tokens", req, t.sl)
	c.Assert(err, IsNil)
	c.Assert(injected, Equals, true)

	requestBody, err := io.ReadAll(req.Body)
	c.Assert(err, IsNil)

	var bodyData map[string]any
	err = json.Unmarshal(requestBody, &bodyData)
	c.Assert(err, IsNil)

	c.Check(bodyData, DeepEquals, map[string]any{
		"auth": map[string]any{
			"identity": map[string]any{
				"methods": []any{"password"},
				"password": map[string]any{
					"user": map[string]any{
						"name":     "new-user",
						"password": "new-pass",
						"domain": map[string]any{
							"name": "my-domain",
						},
					},
				},
			},
			"scope": map[string]any{
				"project": map[string]any{
					"name": "my-project",
					"domain": map[string]any{
						"name": "other-domain",
					},
				},
			},
			"extra-field": "extra-content",
		},
	})
}

func (t *secretSuite) TestInjectBodySecretsApplicationCredential(c *C) {
	sec := []secrets.Secret{
		{Type: secrets.KeystoneV3Type, URL: glob.MustCompile("https://my-domain.com:5000/v3/auth/tokens"), KeystoneV3Creds: "new-id:new-secret"},
	}

	body := []byte(`{
		"auth": {
			"identity": {
				"methods": ["application_credential"],
				"application_credential": { "id": "old-id", "secret": "old-secret" }
			},
			"extra-field": "extra-content"
		}
	}`)

	req, err := http.NewRequest("GET", "https://my-domain.com:5000/v3/auth/tokens", bytes.NewReader(body))
	c.Assert(err, IsNil)

	injected, err := secrets.InjectSecrets(sec, "https://my-domain.com:5000/v3/auth/tokens", req, t.sl)
	c.Assert(err, IsNil)
	c.Assert(injected, Equals, true)

	requestBody, err := io.ReadAll(req.Body)
	c.Assert(err, IsNil)

	var bodyData map[string]any
	err = json.Unmarshal(requestBody, &bodyData)
	c.Assert(err, IsNil)

	c.Check(bodyData, DeepEquals, map[string]any{
		"auth": map[string]any{
			"identity": map[string]any{
				"methods": []any{"application_credential"},
				"application_credential": map[string]any{
					"id":     "new-id",
					"secret": "new-secret",
				},
			},
			"extra-field": "extra-content",
		},
	})
}

func (t *secretSuite) TestInjectBodySecretsRejectsAmbiguousMultiMethodIdentity(c *C) {
	sec := []secrets.Secret{
		{Type: secrets.KeystoneV3Type, URL: glob.MustCompile("https://my-domain.com:5000/v3/auth/tokens"), KeystoneV3Creds: "new-id:new-secret"},
	}

	// Both "password" and "application_credential" are listed. Injection
	// only has one credential pair to substitute, so picking either method
	// and dropping the other would silently change the auth request's
	// meaning (e.g. downgrading a multi-factor requirement). This must be
	// rejected instead of forwarding either interpretation.
	body := []byte(`{
		"auth": {
			"identity": {
				"methods": ["password", "application_credential"],
				"password": { "user": { "name": "old-name", "password": "old-pass", "domain": {"name": "my-domain"} } },
				"application_credential": { "id": "old-id", "secret": "old-secret" }
			}
		}
	}`)

	req, err := http.NewRequest("GET", "https://my-domain.com:5000/v3/auth/tokens", bytes.NewReader(body))
	c.Assert(err, IsNil)

	injected, err := secrets.InjectSecrets(sec, "https://my-domain.com:5000/v3/auth/tokens", req, t.sl)
	c.Assert(injected, Equals, false)
	c.Assert(err, ErrorMatches, ".*exactly one method is supported.*")
}

func (t *secretSuite) TestInjectBodySecretsDispatchesOnMethodsNotFieldPresence(c *C) {
	sec := []secrets.Secret{
		{Type: secrets.KeystoneV3Type, URL: glob.MustCompile("https://my-domain.com:5000/v3/auth/tokens"), KeystoneV3Creds: "new-user:new-pass"},
	}

	// The methods list says "password", but a stray application_credential
	// object is also present. Dispatch must follow methods, not field
	// presence, so this must still be rewritten as password auth.
	body := []byte(`{
		"auth": {
			"identity": {
				"methods": ["password"],
				"password": { "user": { "name": "old-name", "password": "old-pass", "domain": {"name": "my-domain"} } },
				"application_credential": { "id": "stray-id", "secret": "stray-secret" }
			}
		}
	}`)

	req, err := http.NewRequest("GET", "https://my-domain.com:5000/v3/auth/tokens", bytes.NewReader(body))
	c.Assert(err, IsNil)

	injected, err := secrets.InjectSecrets(sec, "https://my-domain.com:5000/v3/auth/tokens", req, t.sl)
	c.Assert(err, IsNil)
	c.Assert(injected, Equals, true)

	requestBody, err := io.ReadAll(req.Body)
	c.Assert(err, IsNil)

	var bodyData map[string]any
	err = json.Unmarshal(requestBody, &bodyData)
	c.Assert(err, IsNil)

	identity := bodyData["auth"].(map[string]any)["identity"].(map[string]any)
	c.Check(identity["methods"], DeepEquals, []any{"password"})
	c.Check(identity["password"], DeepEquals, map[string]any{
		"user": map[string]any{
			"name":     "new-user",
			"password": "new-pass",
			"domain": map[string]any{
				"name": "my-domain",
			},
		},
	})
	_, hasApplicationCredential := identity["application_credential"]
	c.Check(hasApplicationCredential, Equals, false)
}

func (t *secretSuite) TestInjectBodySecretsRejectsMethodWithoutMatchingObject(c *C) {
	sec := []secrets.Secret{
		{Type: secrets.KeystoneV3Type, URL: glob.MustCompile("https://my-domain.com:5000/v3/auth/tokens"), KeystoneV3Creds: "new-id:new-secret"},
	}

	// The methods list says "application_credential", but the object is
	// missing. Forwarding the caller's original, unsubstituted body here
	// would be exactly the silent-failure this feature exists to fix, so
	// the request must be rejected instead.
	body := []byte(`{
		"auth": {
			"identity": {
				"methods": ["application_credential"]
			}
		}
	}`)

	req, err := http.NewRequest("GET", "https://my-domain.com:5000/v3/auth/tokens", bytes.NewReader(body))
	c.Assert(err, IsNil)

	injected, err := secrets.InjectSecrets(sec, "https://my-domain.com:5000/v3/auth/tokens", req, t.sl)
	c.Assert(injected, Equals, false)
	c.Assert(err, ErrorMatches, ".*application_credential object is missing.*")
}

// failingReader returns some data, then a non-EOF error, simulating a
// body read that fails partway through (e.g. a client disconnect).
type failingReader struct {
	data []byte
	err  error
}

func (r *failingReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func (t *secretSuite) TestInjectBodySecretsPropagatesBodyReadError(c *C) {
	sec := []secrets.Secret{
		{Type: secrets.KeystoneV3Type, URL: glob.MustCompile("https://my-domain.com:5000/v3/auth/tokens"), KeystoneV3Creds: "new-id:new-secret"},
	}

	req, err := http.NewRequest("GET", "https://my-domain.com:5000/v3/auth/tokens", nil)
	c.Assert(err, IsNil)

	// Simulate a request whose body read fails partway through. Only a
	// truncated prefix is available, so forwarding it dressed up as a
	// complete request would send malformed JSON to Keystone in place of
	// whatever the client actually sent. The caller must be told to
	// reject the request instead of receiving a fabricated body.
	req.Body = io.NopCloser(&failingReader{data: []byte(`{"partial`), err: errors.New("connection reset by peer")})

	injected, err := secrets.InjectSecrets(sec, "https://my-domain.com:5000/v3/auth/tokens", req, t.sl)
	c.Assert(injected, Equals, false)
	c.Assert(err, ErrorMatches, "cannot read keystone-v3 request body:.*")
}

func (t *secretSuite) TestInjectBodySecretsRejectsOversizedBody(c *C) {
	sec := []secrets.Secret{
		{Type: secrets.KeystoneV3Type, URL: glob.MustCompile("https://my-domain.com:5000/v3/auth/tokens"), KeystoneV3Creds: "new-id:new-secret"},
	}

	// A body far larger than any real Keystone v3 auth-token request
	// must be rejected before being fully buffered in memory.
	oversized := bytes.Repeat([]byte("x"), 2<<20)

	req, err := http.NewRequest("GET", "https://my-domain.com:5000/v3/auth/tokens", bytes.NewReader(oversized))
	c.Assert(err, IsNil)

	injected, err := secrets.InjectSecrets(sec, "https://my-domain.com:5000/v3/auth/tokens", req, t.sl)
	c.Assert(injected, Equals, false)
	c.Assert(errors.Is(err, secrets.ErrKeystoneV3BodyTooLarge), Equals, true)
}

func (t *secretSuite) TestInjectBodySecretsNilBodyIsNoOp(c *C) {
	sec := []secrets.Secret{
		{Type: secrets.KeystoneV3Type, URL: glob.MustCompile("https://my-domain.com:5000/v3/auth/tokens"), KeystoneV3Creds: "new-id:new-secret"},
	}

	// A bodyless request (e.g. GET, or any manually constructed request
	// with a nil Body) matching the secret's URL rule must not panic.
	req, err := http.NewRequest("GET", "https://my-domain.com:5000/v3/auth/tokens", nil)
	c.Assert(err, IsNil)
	c.Assert(req.Body, IsNil)

	injected, err := secrets.InjectSecrets(sec, "https://my-domain.com:5000/v3/auth/tokens", req, t.sl)
	c.Assert(err, IsNil)
	c.Assert(injected, Equals, true)
	c.Check(req.Body, IsNil)
}

type getKeystoneV3IdentityDomainTest struct {
	input  string         // The auth request
	domain map[string]any // Expected domain output
	errMsg string         // Expected error message, if any
}

var getKeystoneV3IdentityDomainTests = []getKeystoneV3IdentityDomainTest{{
	// Valid domain name
	input: `{
			"identity": {
				"methods": ["password"],
				"password": {
					"user": { "domain": { "name": "Default" } }
				}
			}
		}`,
	domain: map[string]any{"name": "Default"},
	errMsg: "",
}, {
	// Valid domain id
	input: `{
			"identity": {
				"methods": ["password"],
				"password": {
					"user": { "domain": { "id": "default" } }
				}
			}
		}`,
	domain: map[string]any{"id": "default"},
	errMsg: "",
}, {
	// Missing identity
	input:  `{ "other": "" }`,
	domain: nil,
	errMsg: "cannot find identity in keystone-v3 auth request",
}, {
	// Missing password
	input: `{
			"identity": {
				"methods": ["password"],
				"other": ""
			}
		}`,
	domain: nil,
	errMsg: "cannot find password in keystone-v3 auth request",
}, {
	// Missing user
	input: `{
			"identity": {
				"methods": ["password"],
				"password": {}
			}
		}`,
	domain: nil,
	errMsg: "cannot find user in keystone-v3 auth request",
}}

func (t *secretSuite) TestGetKeystoneV3IdentityDomain(c *C) {
	for _, tc := range getKeystoneV3IdentityDomainTests {
		var auth map[string]json.RawMessage
		err := json.Unmarshal([]byte(tc.input), &auth)
		c.Assert(err, IsNil)

		domain, err := secrets.GetKeystoneV3IdentityDomain(auth)
		if tc.errMsg == "" {
			c.Assert(err, IsNil)
			c.Assert(domain, DeepEquals, tc.domain)
		} else {
			c.Assert(err, ErrorMatches, tc.errMsg)
		}
	}
}
