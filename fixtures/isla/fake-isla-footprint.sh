#!/bin/sh
# This fixture emits generic trace text from received bytes and addresses.
# Special encodings simulate process, protocol, and output-limit failures.
set -eu

for argument in "$@"; do
	case "$argument" in
	--version)
		printf '%s\n' 'v0.2.0/footprint-test'
		exit 0
		;;
	-s|--simplify-registers|--hide)
		printf '%s\n' 'lossy trace option received' >&2
		exit 4
		;;
	esac
done

instruction=
initial_pc=
while [ "$#" -gt 0 ]; do
	case "$1" in
	-i)
		shift
		instruction=$1
		;;
	-I)
		shift
		initial_pc=$1
		;;
	esac
	shift
done

case "$instruction" in
deaddead)
	printf '%s\n' 'footprint process stopped' >&2
	exit 3
	;;
00000000)
	exit 0
	;;
ffffffff)
	printf '(trace\n  (bytes %01024d)\n)\n' 1
	;;
abababab)
	printf '%s\n' 'No primop external_operation (Name 7)' >&2
	printf '(trace\n  (bytes %s)\n)\n' "$instruction"
	;;
cdcdcdcd)
	printf '%s\n' 'Warning: unknown diagnostic' >&2
	printf '(trace\n  (bytes %s)\n)\n' "$instruction"
	;;
*)
	printf '(trace\n  (bytes %s)\n  (initial %s)\n)\n' "$instruction" "$initial_pc"
	;;
esac
