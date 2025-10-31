#!/bin/bash -e
# Wrapper for running the fetch service using snap configuration.

proxy_port="$(snapctl get proxy.port)"
control_port="$(snapctl get control.port)"
verbosity="$(snapctl get verbosity)"
permissive="$(snapctl get permissive)"
if [[ "${permissive,,}" == "true" ]]; then
	permissive="--permissive-mode"
else
	permissive=""
fi
if [[ "$(snapctl get profile.enabled)" == "true" ]]; then
	profile="--profile"
else
	profile=""
fi
profile_port="$(snapctl get profile.port)"
control_port="$(snapctl get control.port)"
control_auth="$(snapctl get control.auth)"

log_file="$(snapctl get log.file || true)"

FETCH_SERVICE_AUTH="$control_auth"
export FETCH_SERVICE_AUTH

if [[ -z "${log_file}" ]]; then
	exec "${SNAP}/bin/fetch" \
		"--proxy-port=${proxy_port}" \
		"--control-port=${control_port}" \
		"--profile-port=${profile_port}" \
		"--spool=${SNAP_COMMON}/spool" \
		"--config=${SNAP_DATA}/conf" \
		"--cert=${SNAP_DATA}/certs/ca.pem" \
		"--key=${SNAP_DATA}/certs/ca.key.pem" \
		"--verbosity=${verbosity}" \
		${profile} \
		${permissive}
else
	exec "${SNAP}/bin/fetch" \
		"--proxy-port=${proxy_port}" \
		"--control-port=${control_port}" \
		"--profile-port=${profile_port}" \
		"--spool=${SNAP_COMMON}/spool" \
		"--config=${SNAP_DATA}/conf" \
		"--cert=${SNAP_DATA}/certs/ca.pem" \
		"--key=${SNAP_DATA}/certs/ca.key.pem" \
		"--verbosity=${verbosity}" \
		${profile} \
		${permissive} > >(tee -a "${SNAP_DATA}/${log_file}")
fi
