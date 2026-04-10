Pass secrets to Fetch Service
=============================

Builds sometimes need to access authenticated resources, such as private Git 
repositories or protected package servers. Rather than exposing credentials 
to the build environment, you can provide them to Fetch Service, which will 
inject them into requests to specified servers. This keeps credentials 
isolated from the build scripts.

Secrets are per-session and are provided to Fetch Service when creating 
the session.

Create a session with secrets
-----------------------------

Each secret specifies a type, a URL pattern and credentials. The type determines
how the fetch service will inject the credentials into any request that matches 
the pattern.

Use ``basic-auth`` for services that accept HTTP Basic authentication. To access a 
private GitHub repository with the username ``myuser`` and password ``mytoken``:

.. code-block:: bash

    secrets='[{
        "type":"basic-auth", 
        "url": "https://github.com:443/canonical/fetch-service.git/**", 
        "basic-credentials": "myuser:mytoken"
    }]'

    curl -s -X POST \
      -d '{"policy":"permissive", "secrets": '"$secrets"'}' \
      http://user:password@localhost:9999/session


Use ``macaroon`` for services that require macaroon-based authentication. To access 
a package store with a macaroon assigned to a ``$MACAROON`` variable: 

.. code-block:: bash

    secrets='[{
        "type":"macaroon", 
        "url": "https://api.staging.pkg.store:443/**", 
        "macaroon-credentials": "'$MACAROON'"
    }]'

    curl -s -X POST \
      -d '{"policy":"permissive", "secrets": '"$secrets"'}' \
      http://user:password@localhost:9999/session
