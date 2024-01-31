#!/bin/bash -e
# Run some basic tests for the fetch service snap.
# These mostly cover testing the hooks and other scripts.
# To run run these tests, ensure the snap for testing is installed and
# execute this script. Check the return value for success or failure.

echo "Testing that default values of config options get set..."
echo " - proxy.port"
snap unset fetch-service proxy.port
test "$(snap get fetch-service proxy.port)" = "9988"
echo " - verbosity"
snap unset fetch-service verbosity
test "$(snap get fetch-service verbosity)" = "debug"
echo " - permissive"
snap unset fetch-service permissive
test "$(snap get fetch-service permissive)" = "false"

echo "Testing config coercion"
echo " - verbosity (lower case)"
snap set fetch-service verbosity="QUIET"
test "$(snap get fetch-service verbosity)" = "quiet"
echo " - permissive (boolean)"
echo "   (TRUE -> true)"
snap set fetch-service permissive="TRUE"
test "$(snap get fetch-service permissive)" = "true"
echo "   (0 -> false)"
snap set fetch-service permissive="0"
test "$(snap get fetch-service permissive)" = "false"

echo "Testing creation of spool directory"
test -d /var/snap/fetch-service/common/spool/

echo "Testing that service starts and stops correctly"
snap stop fetch-service.fetchd
snap services fetch-service.fetchd | grep -q '\binactive\b'
snap start fetch-service.fetchd
snap services fetch-service.fetchd | grep -q '\bactive\b'
snap stop fetch-service.fetchd
snap services fetch-service.fetchd | grep -q '\binactive\b'
