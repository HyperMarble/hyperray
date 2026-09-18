#!/bin/sh
# This fixture reports one whole-program solver process failure.
set -eu

if [ "${1-}" = "--version" ]; then
	printf '%s\n' 'v0.2.0/footprint-test'
	exit 0
fi

printf '%s\n' 'executable solver stopped' >&2
exit 3
