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

exec "${SNAP}/bin/fetch"\
  "--port=${proxy_port}"\
  "${permissive}"\
  "--spool=${SNAP_COMMON}/spool"\
  "--verbosity=${verbosity}"
