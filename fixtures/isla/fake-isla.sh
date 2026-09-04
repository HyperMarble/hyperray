#!/bin/sh
# This fixture supplies a stable external-tool identity for unit tests.
# It must not represent real Sail or Isla semantics.
set -eu

if [ "${1-}" = "--version" ]; then
	printf '%s\n' 'v0.2.0/test'
	exit 0
fi

printf '%s\n' 'unsupported fixture operation' >&2
exit 2
