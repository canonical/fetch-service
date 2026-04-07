Passing secrets to the fetch service
====================================

When creating a session, you can provide credentials for the fetch 
service to inject when forwarding requests to specified servers.

Create a session with secrets
-----------------------------

Define a secret and create a session::

    secrets='[{
        "type":"basic-auth", 
        "url": "https://github.com:443/canonical/fetch-service.git/**", 
        "basic-credentials": "myuser:mytoken"
    }]'

    curl -s -X POST \
      -d '{"policy":"permissive", "secrets": '"$secrets"'}' \
      http://user:password@localhost:9999/session > session.json

Extract the session ID and token::

    session_id=$(jq -r .id session.json)
    token=$(jq -r .token session.json)

Configure the proxy using the extracted session ID and token::

    export http_proxy="http://${session_id}:${token}@localhost:9988"
    export https_proxy="https://${session_id}:${token}@localhost:9988"

Requests through the proxy will now have credentials injected automatically.

This example uses a ``basic-auth`` secret. For other supported secret types, 
see :doc:`/reference/control-api`.

Clean up
--------
Revoke the token and delete the session::

    unset http_proxy
    unset https_proxy

    curl -sf -X DELETE -d "{\"token\": \"${token}\"}" "http://localhost:9999/session/${session_id}/token"
    curl -sf -X DELETE "http://user:password@localhost:9999/session/${session_id}"
    curl -sf -X DELETE "http://user:password@localhost:9999/resources/${session_id}"

