#!/bin/sh
# This legacy-tool fixture rejects the required executable-entry capability.
# A missing capability must not produce a proof result.
set -eu
if [ "${1-}" = "--version" ]; then
    printf '%s\n' 'v0.2.0/footprint-test'
    exit 0
fi
for argument in "$@"; do
    if [ "$argument" = "--executable-entry" ]; then
        printf '%s\n' 'unrecognized option --executable-entry' >&2
        exit 2
    fi
done
printf '%s\n' 'Test q Forbidden' 'States 1' '???;' 'Positive: 0 Negative: 1'
