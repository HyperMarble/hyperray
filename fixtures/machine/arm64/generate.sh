#!/bin/sh
# This script builds the real Rust-derived thin ARM64 Mach-O fixture.
# It must not run the fixture or claim ordinary macOS process support.
set -eu

fixture_directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
build_directory=$(mktemp -d "${TMPDIR:-/tmp}/hyperray-arm64.XXXXXX")
staging_artifact=$(mktemp "$fixture_directory/.tiny-arm64-static.XXXXXX")
cleanup() {
	rm -f "$staging_artifact"
	rm -rf "$build_directory"
}
trap cleanup EXIT

sdk_version=$(xcrun --sdk macosx --show-sdk-version)
object_file="$build_directory/source.o"
temporary_artifact="$build_directory/tiny-arm64-static"
artifact="$fixture_directory/tiny-arm64-static"
object_symbols="$build_directory/object-symbols.txt"
artifact_file_info="$build_directory/artifact-file-info.txt"
artifact_symbols="$build_directory/artifact-symbols.txt"

rustc --edition=2021 --target aarch64-apple-darwin --crate-type=lib \
	--emit=obj -C panic=abort -C relocation-model=static \
	-C codegen-units=1 -O "$fixture_directory/source.rs" -o "$object_file"
xcrun nm -g "$object_file" > "$object_symbols"
exported_symbols=$(awk '$2 ~ /^[Tt]$/ {print $3}' "$object_symbols")
exported_symbol_count=$(awk '$2 ~ /^[Tt]$/ {count++} END {print count + 0}' "$object_symbols")
if [ "$exported_symbol_count" -ne 1 ]; then
	echo "expected one exported text symbol, found $exported_symbol_count" >&2
	exit 1
fi
exported_symbol=$exported_symbols

xcrun ld -arch arm64 -static -no_pie \
	-platform_version macos "$sdk_version" "$sdk_version" \
	-e "$exported_symbol" -undefined error -o "$temporary_artifact" "$object_file"
cp "$temporary_artifact" "$staging_artifact"
xcrun file "$staging_artifact" > "$artifact_file_info"
grep -F 'Mach-O 64-bit executable arm64' "$artifact_file_info"
xcrun nm -arch arm64 -g "$staging_artifact" > "$artifact_symbols"
linked_symbol_count=$(awk '$2 ~ /^[Tt]$/ {count++} END {print count + 0}' "$artifact_symbols")
if [ "$linked_symbol_count" -ne 1 ]; then
	echo "expected one linked text symbol, found $linked_symbol_count" >&2
	exit 1
fi
linked_symbol=$(awk '$2 ~ /^[Tt]$/ {print $3}' "$artifact_symbols")
if [ "$linked_symbol" != "$exported_symbol" ]; then
	echo "linked symbol $linked_symbol does not match $exported_symbol" >&2
	exit 1
fi
temporary_digest=$(shasum -a 256 "$staging_artifact")
printf '%s\n' "$temporary_digest"
mv "$staging_artifact" "$artifact"
