#!/bin/sh
# This fixture emits complete and malformed program semantic reports.
# It tests protocol checks and does not represent real Isla semantics.
set -eu

if [ "${1-}" = "--version" ]; then
	printf '%s\n' 'v0.2.0/test'
	exit 0
fi

program=
for argument in "$@"; do
	program=$argument
done
mode=$(sed -n '1p' "$program")

emit_tree() {
	printf '%s\n' 'Thread 0:' '(trace'
	printf '%s\n' '  (read-reg |PC| nil #x0000000000001000)'
	printf '%s\n' '  (instr #x00300293)' '  (write-reg |x5| nil #x03)' ')'
}

emit_footer() {
	printf '%s\n' 'Final Assertion:' 'Not(Eq(x5, 3))' 'Memory:test'
	printf '%s\n' 'opcode 00300293, Footprint:' '  Register writes: x5'
}

case "$mode" in
proof | counterexample | tool-error | visit-limit | malformed | process-error | proposal-warning | solver-timeout)
	emit_tree
	emit_footer
	;;
semantic-missing)
	emit_tree
	printf '%s\n' 'Final Assertion:' 'Not(Eq(x5, 3))' 'Memory:test'
	;;
semantic-extra)
	emit_tree
	emit_footer
	printf '%s\n' 'opcode 00000013, Footprint:'
	;;
semantic-duplicate)
	emit_tree
	emit_footer
	printf '%s\n' 'opcode 00300293, Footprint:'
	;;
semantic-malformed)
	printf '%s\n' 'Thread 0:' '(trace' '  (read-reg |PC| nil #x0000000000001000)' '  (instr #x00300293)'
	emit_footer
	;;
semantic-warning)
	emit_tree
	emit_footer
	printf '%s\n' 'unexpected warning' >&2
	;;
semantic-process-error)
	printf '%s\n' 'semantic process stopped' >&2
	exit 3
	;;
semantic-change-program)
	emit_tree
	emit_footer
	printf '%s\n' 'changed' > "$program"
	;;
semantic-change-tool)
	emit_tree
	emit_footer
	printf '%s\n' '# changed' >> "$0"
	;;
*)
	printf '%s\n' 'unknown semantic fixture input' >&2
	exit 2
	;;
esac
