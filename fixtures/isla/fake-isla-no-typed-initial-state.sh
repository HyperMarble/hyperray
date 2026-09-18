#!/bin/sh
# An old tool must reject the requested typed-state capability.
# A missing input feature must never produce a proof result.
set -eu
if [ "${1-}" = "--version" ]; then
    printf '%s\n' 'v0.2.0/footprint-test'
    exit 0
fi
for argument in "$@"; do
    if [ "$argument" = "--typed-initial-state" ]; then
        printf '%s\n' 'unrecognized option --typed-initial-state' >&2
        exit 2
    fi
done
printf '%s\n' 'typed-state capability was not requested' >&2
exit 3
