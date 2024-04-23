#!/bin/bash -e
# Wrapper for running the fetch service using snap configuration.

proxy_port="$(snapctl get proxy.port)"
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


keyring="/usr/share/keyrings/ubuntu-archive-keyring.gpg"
key="F6ECB3762474EDA9D21B7022871920D1991BC93C"
export FETCH_APT_RELEASE_PUBLIC_KEY=$(gpg --export --armor --no-default-keyring --keyring "$keyring" "$key")
export FETCH_SERVICE_AUTH="$control_auth"

exec "${SNAP}/bin/fetch"\
  "--proxy-port=${proxy_port}"\
  "--spool=${SNAP_COMMON}/spool"\
  "--verbosity=${verbosity}"\
  "--control-port=${control_port}"\
  "--profile-port=${profile_port}"\
  ${profile} ${permissive}
