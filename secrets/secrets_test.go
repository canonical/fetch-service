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
		// Good secret
		{[]secrets.Secret{{Type: secrets.BasicAuthType, URL: glob.MustCompile("www.example.com"), BasicCreds: "user:passwd"}}, nil},
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
        }
      ]
    }`)

	var j testGlob
	err := json.Unmarshal(data, &j)
	c.Assert(err, IsNil)

	c.Assert(len(j.Secrets), Equals, 1)
	c.Assert(j.Secrets[0].Type, Equals, secrets.BasicAuthType)
	c.Assert(j.Secrets[0].URL, DeepEquals, glob.MustCompile("https://github.com:443/canonical/fetch-service.git/**"))
	c.Assert(j.Secrets[0].BasicCreds, Equals, "user:passwd")
}

func (t *secretSuite) TestInjectSecrets(c *C) {
	sec := []secrets.Secret{
		{Type: secrets.BasicAuthType, URL: glob.MustCompile("https://github.com:443/canonical/fetch-service.git/**"), BasicCreds: "user:passwd"},
	}

	for _, tc := range []struct {
		url      string
		injected bool
		header   string
	}{
		{"www.example.com", false, ""},
		{"https://github.com:443/canonical/different-repo.git/", false, ""},
		{"https://github.com:443/canonical/fetch-service.git/", true, "Basic dXNlcjpwYXNzd2Q="},
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
