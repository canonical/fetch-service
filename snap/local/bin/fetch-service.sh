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
log_file="$(snapctl get log.file)"

if [[ -z "${log_file}" ]]; then
  exec "${SNAP}/bin/fetch"\
    "--port=${proxy_port}"\
    "${permissive}"\
    "--spool=${SNAP_COMMON}/spool"\
    "--verbosity=${verbosity}"
else
  exec "${SNAP}/bin/fetch"\
    "--port=${proxy_port}"\
    "${permissive}"\
    "--spool=${SNAP_COMMON}/spool"\
    "--verbosity=${verbosity}" >  >(tee -a "${SNAP_DATA}/${log_file}")
fi
