#!/bin/sh
# Exercise observer transport and limits with shell builtins only.
# These outputs are process contracts, not program-correctness evidence.
case "$1" in
  silent)
    exit 0
    ;;
  streams)
    printf 'worker output\n'
    printf 'worker diagnostic\n' >&2
    exit 0
    ;;
  failure)
    printf 'declared worker error\n' >&2
    exit 7
    ;;
  flood)
    while :; do printf 'output exceeds the declared limit\n'; done
    ;;
  wait)
    while :; do :; done
    ;;
  environment)
    printf '%s\n' "${HYPERRAY_OBSERVER_TEST-unset}"
    ;;
  *)
    printf 'unknown fixture mode\n' >&2
    exit 2
    ;;
esac
