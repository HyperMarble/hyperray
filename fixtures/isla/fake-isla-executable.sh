#!/bin/sh
# This fixture returns one solver proposal for generated executable tests.
set -eu

if [ "${1-}" = "--version" ]; then
	printf '%s\n' 'v0.2.0/footprint-test'
	exit 0
fi

printf '%s\n' 'Test generated-code Forbidden' 'States 1'
printf '%s\n' '???;' 'No' 'Witnesses' 'Positive: 0 Negative: 1'
