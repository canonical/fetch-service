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
upstream_http_proxy="$(snapctl get upstream-proxy.http)"
upstream_https_proxy="$(snapctl get upstream-proxy.https)"
upstream_proxy_bypass="$(snapctl get upstream-proxy.bypass)"

log_file="$(snapctl get log.file || true)"

FETCH_SERVICE_AUTH="$control_auth"
export FETCH_SERVICE_AUTH

if [[ -n "${upstream_http_proxy}" ]]; then
	HTTP_PROXY="${upstream_http_proxy}"
	export HTTP_PROXY
fi

if [[ -n "${upstream_https_proxy}" ]]; then
	HTTPS_PROXY="${upstream_https_proxy}"
	export HTTPS_PROXY
fi

if [[ -n "${upstream_proxy_bypass}" ]]; then
	NO_PROXY="${upstream_proxy_bypass}"
	export NO_PROXY
fi

if [[ -z "${log_file}" ]]; then
	# shellcheck disable=SC2086
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
	# shellcheck disable=SC2086
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
