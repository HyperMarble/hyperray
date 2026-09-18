#!/bin/sh
# An old tool must reject a requested model-call capability.
# It must never supply a success result for an unknown safety condition.
set -eu
if [ "${1-}" = "--version" ]; then
    printf '%s\n' 'v0.2.0/footprint-test'
    exit 0
fi
for argument in "$@"; do
    if [ "$argument" = "--forbidden-model-calls" ]; then
        printf '%s\n' 'unrecognized option --forbidden-model-calls' >&2
        exit 2
    fi
done
printf '%s\n' 'model-call capability was not requested' >&2
exit 3
