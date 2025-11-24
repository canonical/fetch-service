package secrets_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/secrets"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type secretSuite struct{}

var _ = Suite(&secretSuite{})

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
        }
      ]
    }`)

	var j testGlob
	err := json.Unmarshal(data, &j)
	c.Assert(err, IsNil)

	c.Assert(len(j.Secrets), Equals, 2)

	c.Assert(j.Secrets[0].Type, Equals, secrets.BasicAuthType)
	c.Assert(j.Secrets[0].URL, DeepEquals, glob.MustCompile("https://github.com:443/canonical/fetch-service.git/**"))
	c.Assert(j.Secrets[0].BasicCreds, Equals, "user:passwd")

	c.Assert(j.Secrets[1].Type, Equals, secrets.MacaroonType)
	c.Assert(j.Secrets[1].URL, DeepEquals, glob.MustCompile("https://www.example.com/**"))
	c.Assert(j.Secrets[1].MacaroonCreds, Equals, "deadbeef")
}

func (t *secretSuite) TestInjectSecrets(c *C) {
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

		injected := secrets.InjectSecrets(sec, tc.url, req)
		c.Assert(injected, Equals, tc.injected)
		if injected {
			header := req.Header.Get("Authorization")
			c.Assert(header, Equals, tc.header)
		}
	}
}
