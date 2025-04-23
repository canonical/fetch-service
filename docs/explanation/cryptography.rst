.. _explanation-cryptographic-technology:

Cryptographic technology in the Fetch Service
=============================================

The Fetch Service is a proxy server that inspects and authorizes requests to remote
servers and the responses to those requests. Each component in this architecture
makes use of cryptographic processes.

Server certificate
------------------

The Fetch Service makes use of a X.509 certificate to set up the proxy server and
communicate with clients on their requests. The service can use any correctly-configured
certificate, but for convenience the Fetch Service snap creates a certificate during
installation using the ``openssl`` utility present on the core22 snap. The
service itself makes use of the certificate during its operation through the standard
`crypto/tls`_ and `crypto/x509`_ Go packages.

Proxy server sessions
---------------------

The Fetch Service uses the `GoProxy`_ library to implement the proxy server. Before
handling client requests, the client must first ask the service to create a session, which
is used to group requests from the same client and isolate requests between clients.
The service creates an unique session token that must be used by the client during
requests. This token is an alphanumeric value generated with the standard `math/rand`_
Go package.

Response processing
-------------------

The type of processing performed on remote server responses depends on the nature of
the response. What's relevant here is the handling of responses from Apt
package repositories. The contents of these repositories are signed with a key that
is validated by the Fetch Service via the `github.com/ProtonMail/go-crypto`_ Go
library.

.. _GoProxy: https://github.com/elazarl/goproxy
.. _crypto/tls: https://pkg.go.dev/crypto/tls
.. _crypto/x509: https://pkg.go.dev/crypto/x509
.. _github.com/ProtonMail/go-crypto: https://github.com/ProtonMail/go-crypto
.. _math/rand: https://pkg.go.dev/math/rand
