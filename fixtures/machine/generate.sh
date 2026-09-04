#!/bin/sh
# This script builds the real ELF fixtures with the Rust LLVM tools.
# It must run on the recorded Apple Silicon toolchain without a cross SDK.
set -eu

fixture_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
fixture_build_directory=$(mktemp -d /tmp/hyperray-machine.XXXXXX)
rust_tool_bin=$(rustc --print sysroot)/lib/rustlib/aarch64-apple-darwin/bin
llc=$rust_tool_bin/llc
linker=$rust_tool_bin/rust-lld

trap 'rm -rf "$fixture_build_directory"' EXIT

"$llc" -filetype=obj -mtriple=riscv64-unknown-linux-gnu -mattr=+m,+a,+f,+d,+c -target-abi=lp64d "$fixture_directory/canonical.ll" -o "$fixture_build_directory/lp64d.o"
"$llc" -filetype=obj -mtriple=riscv64-unknown-linux-gnu -mattr=+m,+a,+c -target-abi=lp64 "$fixture_directory/canonical.ll" -o "$fixture_build_directory/lp64.o"
"$llc" -filetype=obj -mtriple=riscv32-unknown-linux-gnu -mattr=+m,+a,+f,+d,+c -target-abi=ilp32d "$fixture_directory/canonical.ll" -o "$fixture_build_directory/rv32.o"
"$llc" -filetype=obj -mtriple=riscv64-unknown-linux-gnu -mattr=+m,+a,+f,+d,-c -target-abi=lp64d "$fixture_directory/canonical.ll" -o "$fixture_build_directory/no-rvc.o"

for name in truncated long odd; do
	"$llc" -filetype=obj -mtriple=riscv64-unknown-linux-gnu -mattr=+m,+a,+f,+d,+c -target-abi=lp64d "$fixture_directory/$name.ll" -o "$fixture_build_directory/$name.o"
done

"$llc" -filetype=obj -mtriple=powerpc64-unknown-linux-gnu "$fixture_directory/odd.ll" -o "$fixture_build_directory/ppc64.o"
"$linker" -flavor gnu -m elf64ppc -static -e _start "$fixture_build_directory/ppc64.o" -o "$fixture_directory/ppc64-big-static.elf"
"$linker" -flavor gnu -m elf64lriscv -static --no-relax -T "$fixture_directory/layout.ld" "$fixture_build_directory/lp64d.o" -o "$fixture_directory/rv64-lp64d-static.elf"
"$linker" -flavor gnu -m elf64lriscv -static --no-relax -T "$fixture_directory/layout.ld" "$fixture_build_directory/lp64.o" -o "$fixture_directory/rv64-lp64-static.elf"
"$linker" -flavor gnu -m elf32lriscv -static --no-relax -T "$fixture_directory/layout.ld" "$fixture_build_directory/rv32.o" -o "$fixture_directory/rv32-ilp32d-static.elf"
"$linker" -flavor gnu -m elf64lriscv -static --no-relax -T "$fixture_directory/layout.ld" "$fixture_build_directory/no-rvc.o" -o "$fixture_directory/rv64-lp64d-no-rvc.elf"
"$linker" -flavor gnu -m elf64lriscv -static --no-relax -T "$fixture_directory/writable-code.ld" "$fixture_build_directory/lp64d.o" -o "$fixture_directory/rv64-lp64d-writable-code.elf"
"$linker" -flavor gnu -m elf64lriscv -static --no-relax -T "$fixture_directory/no-code.ld" "$fixture_build_directory/lp64d.o" -o "$fixture_directory/rv64-lp64d-no-code.elf"
"$linker" -flavor gnu -m elf64lriscv -e _start --dynamic-linker /lib/ld-linux-riscv64-lp64d.so.1 "$fixture_build_directory/lp64d.o" -o "$fixture_directory/rv64-lp64d-dynamic.elf"

for name in truncated long odd; do
	"$linker" -flavor gnu -m elf64lriscv -static --no-relax -T "$fixture_directory/layout.ld" "$fixture_build_directory/$name.o" -o "$fixture_directory/rv64-lp64d-$name.elf"
done

shasum -a 256 "$fixture_directory"/*.elf
