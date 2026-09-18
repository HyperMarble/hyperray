#!/bin/sh
# This fixture emulates named Z3 process results for public error-path tests.
# It never supplies a proof record.
if [ "$1" = "--version" ]; then
  echo "Z3 version fixture"
  exit 0
fi
fixture_input=$(sed -n '1,$p')
case "$HYPERRAY_Z3_FIXTURE_MODE" in
  process_error)
    echo "fixture process error"
    exit 7
    ;;
  unsupported)
    echo "maybe"
    exit 0
    ;;
  unknown)
    echo "unknown"
    exit 0
    ;;
  empty)
    exit 0
    ;;
  status_extra)
    echo "sat extra"
    exit 0
    ;;
esac
case "$fixture_input" in
  *get-value*)
    ;;
  *)
    echo "sat"
    exit 0
    ;;
esac
case "$HYPERRAY_Z3_FIXTURE_MODE" in
  witness_error)
    echo "fixture witness error"
    exit 8
    ;;
  changed)
    echo "unsat"
    exit 0
    ;;
esac
echo "sat"
case "$HYPERRAY_Z3_FIXTURE_MODE" in
  malformed_outer) echo "bad" ;;
  malformed_short) echo '((|state.balance|))' ;;
  malformed_pair) echo '(bad one two three four)' ;;
  incomplete) echo '((|state.balance| #x00))' ;;
  invalid)
    echo '((|state.balance| zero) (|input.cost| #x00) (|reference.next.0| #x00) (|candidate.next.0| #x00) (|reference.observation.0| false) (|candidate.observation.0| true))'
    ;;
  duplicate)
    echo '((|state.balance| #x00) (|state.balance| #x00) (|input.cost| #x00)'
    echo ' (|reference.next.0| #x00) (|candidate.next.0| #x00)'
    echo ' (|reference.observation.0| false) (|candidate.observation.0| true))'
    ;;
  extra)
    echo '((|state.balance| #x00) (|input.cost| #x00) (|extra| #x00)'
    echo ' (|reference.next.0| #x00) (|candidate.next.0| #x00)'
    echo ' (|reference.observation.0| false) (|candidate.observation.0| true))'
    ;;
  equal)
    echo '((|state.balance| #x00) (|input.cost| #x00)'
    echo ' (|reference.next.0| #x00) (|candidate.next.0| #x00)'
    echo ' (|reference.observation.0| false) (|candidate.observation.0| false))'
    ;;
esac
