#!/bin/sh
# An old tool must reject the required sequential-memory capability.
# Missing support must not become a proof result.
set -eu
if [ "${1-}" = "--version" ]; then
    printf '%s\n' 'v0.2.0/footprint-test'
    exit 0
fi
for argument in "$@"; do
    if [ "$argument" = "--sequential-memory" ]; then
        printf '%s\n' 'unrecognized option --sequential-memory' >&2
        exit 2
    fi
done
printf '%s\n' 'Test q Forbidden' 'States 1' '???;' 'Positive: 0 Negative: 1'
