.. meta::
    :description: Overview of the security of the Fetch Service, covering implementation specifics such as cryptography and certificates.

:relatedlinks: https://www.rfc-editor.org/rfc/rfc7617

.. _explanation-security:

Fetch Service security
======================

Internally, the Fetch Service is a proxy server that inspects and authorizes requests to
remote servers and the responses to those requests. This document explains how the service's
secrets are securely stored, and how its data is secured at rest and in transit.

.. _explanation-cryptographic-technology:

Cryptographic technology
------------------------

Each component in the Fetch Service's architecture makes use of cryptographic processes.

Server certificate
~~~~~~~~~~~~~~~~~~

The Fetch Service makes use of a `X.509 certificate`_ to set up the proxy server and
inspect HTTPS requests from clients. The service can use any correctly-configured
certificate, but for convenience the Fetch Service snap creates a certificate during
installation using the ``openssl`` utility present on the base snap. The service itself
makes use of the certificate during its operation through the standard `crypto/tls`_ and
`crypto/x509`_ Go packages.

Proxy server sessions
~~~~~~~~~~~~~~~~~~~~~

The Fetch Service uses the `elazarl/goproxy`_ library to implement the proxy server. Before
handling client requests, the client must first ask the service to create a session, which
is used to group requests from the same client and isolate requests between clients.
The service creates an unique session token that must be used by the client during
requests. This token is an alphanumeric value generated with the standard `math/rand`_
Go package.

Response processing
~~~~~~~~~~~~~~~~~~~

The type of processing performed on remote server responses depends on the nature of
the response. What's relevant here is the handling of responses from Apt
package repositories. The contents of these repositories are signed with a key that
is validated by the Fetch Service via the `github.com/ProtonMail/go-crypto`_ Go
library.

.. _explanation-session-secrets:

Session secrets
---------------

In addition to inspecting requests to external servers and their responses, the Fetch
Service supports injecting authorization data like passwords and tokens into the requests.
This option is useful in cases where the clients accessing the servers shouldn't have
access to these credentials.

Credentials are configured on a per-session basis and are referred to as session secrets.
They must be provided when creating a session, and as detailed in the
:ref:`reference for the session creation API endpoint<reference-control-post-session>`
the following types of credentials are supported:

* The Basic HTTP Authentication Scheme. This is the standard "user and password" scheme
  and is typically used when accessing remote Git repositories.
* Macaroons, which are special Cookies used by the Snap Store.

Each provided credential is paired with a match pattern for a web URL, which can contain
wildcards. When processing a request, the Fetch Service will inject the credential if the
request's web address matches the locator. For example, the locator "https://www.github.com/example/\*"
will match a request to "https://www.github.com/example/my-repo.git" but won't match a request to
"https://www.github.com/other-org/other-repo.git".

The Fetch Service doesn't inspect or modify the provided secrets. These secrets are only
kept in memory and aren't included in the artifact metadata generated when finishing sessions.
They aren't logged, stored on disk or transferred to other services.

.. _elazarl/goproxy: https://github.com/elazarl/goproxy
.. _X.509 certificate: https://learn.microsoft.com/en-us/azure/iot-hub/reference-x509-certificates
.. _crypto/tls: https://pkg.go.dev/crypto/tls
.. _crypto/x509: https://pkg.go.dev/crypto/x509
.. _github.com/ProtonMail/go-crypto: https://github.com/ProtonMail/go-crypto
.. _math/rand: https://pkg.go.dev/math/rand
