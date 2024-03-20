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


exec "${SNAP}/bin/fetch"\
  "--proxy-port=${proxy_port}"\
  "--spool=${SNAP_COMMON}/spool"\
  "--verbosity=${verbosity}"\
  "--control-port=${control_port}"\
  "--profile-port=${profile_port}"\
  ${profile} ${permissive}
