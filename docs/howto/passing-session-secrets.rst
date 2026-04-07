Passing secrets to the fetch service
====================================

When creating a session, you can provide credentials for the fetch 
service to inject when forwarding requests to specified servers.

Basic Auth Secrets
------------------

Create a session with a ``basic-auth`` secret::

    creds="username:${TOKEN}"
    secrets='[{
        "type":"basic-auth", 
        "url": "https://github.com:443/canonical/fetch-service.git/**", 
        "basic-credentials": "'$creds'"
    }]'

    curl -s -X POST \
      -d '{"policy":"permissive", "secrets": '"$secrets"'}' \
      http://localhost:9999/session > session.json

Extract the session ID and token::

    session_id=$(jq -r .id session.json)
    token=$(jq -r .token session.json)

Configure the proxy using the extracted session ID and token::

    export http_proxy="http://${session_id}:${token}@localhost:9988"
    export https_proxy="https://${session_id}:${token}@localhost:9988"

Requests through the proxy will now have credentials injected automatically.
