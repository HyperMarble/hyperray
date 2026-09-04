#!/bin/sh
# This fixture supplies a stable external-tool identity for unit tests.
# It must not represent real Sail or Isla semantics.
set -eu

if [ "${1-}" = "--version" ]; then
	printf '%s\n' 'v0.2.0/test'
	exit 0
fi

program=
for argument in "$@"; do
	program=$argument
done

case "$(sed -n '1p' "$program")" in
counterexample)
	printf '%s\n' 'Test different-code Allowed' 'States 1'
	printf '%s\n' 'x5=#x0000000000000003;' 'Ok' 'Witnesses'
	printf '%s\n' 'Positive: 1 Negative: 0'
	;;
proof)
	printf '%s\n' 'Test correct-code Forbidden' 'States 1'
	printf '%s\n' '???;' 'No' 'Witnesses' 'Positive: 0 Negative: 1'
	;;
tool-error)
	printf '%s\n' 'symbolic execution stopped' >&2
	printf '%s\n' 'Test broken Error'
	;;
visit-limit)
	printf '%s\n' 'program-counter visit limit reached' >&2
	printf '%s\n' 'Test bounded Error'
	;;
malformed)
	printf '%s\n' 'not a Herd result'
	;;
process-error)
	printf '%s\n' 'process stopped' >&2
	exit 3
	;;
*)
	printf '%s\n' 'unknown fixture input' >&2
	exit 2
	;;
esac
