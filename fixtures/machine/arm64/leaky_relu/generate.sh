#!/bin/sh
# This script builds and records the real Rust-derived ARM64 packet fixture.
# It must not execute the fixture or imply ordinary macOS process support.
set -eu

fixture_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
build_directory=$(mktemp -d "${TMPDIR:-/tmp}/hyperray-arm64-leaky-relu.XXXXXX")
cleanup() {
	rm -rf "$build_directory"
}
trap cleanup EXIT

sdk_version=$(xcrun --sdk macosx --show-sdk-version)
object_file="$build_directory/source.o"
temporary_artifact="$build_directory/leaky-relu-arm64-static"
source_file="$fixture_directory/source.rs"
artifact="$fixture_directory/leaky-relu-arm64-static"

rustc --edition=2021 --target aarch64-apple-darwin --crate-type=lib \
	--emit=obj -C panic=abort -C relocation-model=static \
	-C codegen-units=1 -O "$source_file" -o "$object_file"
xcrun nm -g "$object_file" > "$build_directory/object-symbols.txt"
exported_symbol=$(awk '$3 == "_arm64_leaky_relu" {print $3}' "$build_directory/object-symbols.txt")
if [ "$exported_symbol" != "_arm64_leaky_relu" ]; then
	echo "missing declared packet entry symbol" >&2
	exit 1
fi

xcrun ld -arch arm64 -static -no_pie \
	-platform_version macos "$sdk_version" "$sdk_version" \
	-e "$exported_symbol" -undefined error -o "$temporary_artifact" "$object_file"
xcrun file "$temporary_artifact" | grep -F 'Mach-O 64-bit executable arm64'
xcrun nm -arch arm64 -g "$temporary_artifact" > "$build_directory/artifact-symbols.txt"
linked_symbol=$(awk '$3 == "_arm64_leaky_relu" {print $3}' "$build_directory/artifact-symbols.txt")
if [ "$linked_symbol" != "$exported_symbol" ]; then
	echo "linked symbol $linked_symbol does not match $exported_symbol" >&2
	exit 1
fi

xcrun otool -tvV "$temporary_artifact" > "$build_directory/disassembly.txt"
cp "$object_file" "$fixture_directory/source.o"
cp "$temporary_artifact" "$artifact"
cp "$build_directory/disassembly.txt" "$fixture_directory/disassembly.txt"

python3 - "$fixture_directory" "$artifact" "$exported_symbol" <<'PY'
import hashlib
import json
import subprocess
import sys

fixture_directory, artifact, symbol = sys.argv[1:]
load_commands = subprocess.check_output(["xcrun", "otool", "-l", artifact], text=True)
lines = load_commands.splitlines()
text = {}
for index, line in enumerate(lines):
    if line.strip() == "sectname __text":
        fields = {parts[0]: parts[1] for parts in (lines[index + offset].split() for offset in range(1, 7)) if len(parts) == 2}
        text = {"addr": int(fields["addr"], 16), "offset": int(fields["offset"]), "size": int(fields["size"], 16)}
        break
if not text:
    raise SystemExit("missing __TEXT,__text section")
symbols = subprocess.check_output(["xcrun", "nm", "-arch", "arm64", "-n", artifact], text=True).splitlines()
function_start = None
for line in symbols:
    parts = line.split()
    if len(parts) == 3 and parts[2] == symbol:
        function_start = int(parts[0], 16)
        break
if function_start is None:
    raise SystemExit(f"missing symbol {symbol}")

def digest(name):
    with open(f"{fixture_directory}/{name}", "rb") as stream:
        return hashlib.sha256(stream.read()).hexdigest()

manifest = {
    "source_sha256": digest("source.rs"),
    "object_sha256": digest("source.o"),
    "macho_sha256": digest("leaky-relu-arm64-static"),
    "disassembly_sha256": digest("disassembly.txt"),
    "function_symbol": symbol,
    "function_start": hex(function_start),
    "function_end": hex(text["addr"] + text["size"]),
    "text_offset": text["offset"],
    "text_size": text["size"],
}
with open(f"{fixture_directory}/manifest.json", "w", encoding="utf-8") as stream:
    json.dump(manifest, stream, indent=2, sort_keys=True)
    stream.write("\n")
print(json.dumps(manifest, indent=2, sort_keys=True))
PY
