package secrets_test

import (
	"testing"

	"github.com/canonical/fetch-service/glob"
	"github.com/canonical/fetch-service/secrets"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type sessionSuite struct{}

var _ = Suite(&sessionSuite{})

func (t *sessionSuite) TestValidateSecrets(c *C) {
	for _, tc := range []struct {
		sec []secrets.Secret
		err error
	}{
		// No secrets
		{nil, nil},
		// Good secret
		{[]secrets.Secret{{Type: "basic-auth", Url: glob.MustCompile("www.example.com")}}, nil},
		// Missing type
		{[]secrets.Secret{{}}, secrets.ErrMissingSecretType},
		// Invalid type
		{[]secrets.Secret{{Type: "invalid-type"}}, secrets.ErrInvalidSecretType},
		// Missing url
		{[]secrets.Secret{{Type: "basic-auth"}}, secrets.ErrMissingSecretUrl},
	} {
		err := secrets.ValidateSecrets(tc.sec)
		c.Assert(err, Equals, tc.err)
	}
}
