Pass secrets to the fetch service
====================================

Builds sometimes need to access authenticated resources, such as private Git 
repositories or protected package servers. Rather than exposing credentials 
to the build environment, you can provide them to the fetch service, which 
will inject them into matching requests. This keeps requests isolated from
the build scripts.

Secrets are per-session and are provided to the fetch service when creating 
the session.

Create a session with secrets
-----------------------------

Define a secret with a URL pattern and credentials. The fetch service will 
inject the credentials into any request that matches the pattern.

To access a private Github repository with username "myuser" and password "mytoken":

.. code-block:: bash

    secrets='[{
        "type":"basic-auth", 
        "url": "https://github.com:443/canonical/fetch-service.git/**", 
        "basic-credentials": "myuser:mytoken"
    }]'

    curl -s -X POST \
      -d '{"policy":"permissive", "secrets": '"$secrets"'}' \
      http://user:password@localhost:9999/session


To access a package store with a macaroon stored in ``$MACAROON``: 

.. code-block:: bash

    secrets='[{
        "type":"macaroon", 
        "url": "https://api.staging.pkg.store:443/**", 
        "macaroon-credentials": "'$MACAROON'"
    }]'

    curl -s -X POST \
      -d '{"policy":"permissive", "secrets": '"$secrets"'}' \
      http://user:password@localhost:9999/session
