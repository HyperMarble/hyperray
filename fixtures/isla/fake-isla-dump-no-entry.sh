#!/bin/sh
# This protocol fixture omits the mandatory entry from its execution inventory.
# It must not receive an executable result even with valid static footprints.
set -eu

if [ "${1-}" = "--version" ]; then
	printf '%s\n' 'v0.2.0/footprint-test'
	exit 0
fi

printf '%s\n' 'Thread 0:' '(trace'
printf '%s\n' '  (read-reg |PC| nil #x0000000080100004)' '  (instr #x459d)'
printf '%s\n' '  (read-reg |PC| nil #x0000000080100006)' '  (instr #x00b50023)'
printf '%s\n' '  (read-reg |PC| nil #x000000008010000a)' '  (instr #x8082)' ')'
printf '%s\n' 'Final Assertion:' 'Not(Eq(x11, 7))' 'Memory:test'
printf '%s\n' 'opcode 0000459d, Footprint:' 'opcode 00b50023, Footprint:' 'opcode 00008082, Footprint:'
