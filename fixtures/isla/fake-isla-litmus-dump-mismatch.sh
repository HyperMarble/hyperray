#!/bin/sh
# This fixture reports a different version for release-mismatch tests.
# Verification must stop before it requests semantic output.
set -eu

printf '%s\n' 'v0.2.1/test'
