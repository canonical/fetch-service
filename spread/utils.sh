create_session() (
    local policy secrets body tmpfile session_id token
    exec 3>&1
    exec 1>&2

    policy="$1"
    secrets="${2:-}"
    if [ -n "${secrets}" ]; then
        body='{"policy": "'"${policy}"'", "secrets": '"${secrets}"'}'
    else
        body='{"policy": "'"${policy}"'"}'
    fi
    tmpfile=$(mktemp)
    curl -s -X POST -d "${body}" http://craft:craft@localhost:9999/session | tee "${tmpfile}"
    session_id=$(jq -r .id "${tmpfile}")
    token=$(jq -r .token "${tmpfile}")
    rm "${tmpfile}"

    echo "${session_id} ${token}" >&3
)

revoke_token() (
    local tmpfile session_id token spool_path
    exec 3>&1
    exec 1>&2

    session_id="$1"
    token="$2"
    tmpfile=$(mktemp)
    curl -s -X DELETE -d "{\"token\": \"${token}\"}" "http://localhost:9999/session/${session_id}/token" | tee "${tmpfile}"
    spool_path=$(jq -r '."spool-path"' "${tmpfile}")
    rm "${tmpfile}"

    echo "${spool_path}" >&3
)

get_status() {
    curl -s http://localhost:9999/status
}

get_session_report() {
    local session_id="$1"
    curl -s "http://craft:craft@localhost:9999/session/${session_id}"
}

delete_session() {
    local session_id="$1"
    curl -s -X DELETE "http://craft:craft@localhost:9999/session/${session_id}"
}

delete_resources() {
    local session_id="$1"
    curl -s -X DELETE "http://craft:craft@localhost:9999/resources/${session_id}"
}
