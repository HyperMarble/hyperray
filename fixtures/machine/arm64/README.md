# Native ARM64 Mach-O fixture

This directory contains a real Rust-derived thin ARM64 Mach-O executable.
The artifact is for header analysis. It is not ordinary macOS process support.
The static link is kernel-oriented and does not establish dyld or runtime support.

The source is `source.rs`. The separate generation command is:

```text
fixtures/machine/arm64/generate.sh
```

The script compiles the source with these Rust options:

```text
rustc --edition=2021 --target aarch64-apple-darwin --crate-type=lib --emit=obj -C panic=abort -C relocation-model=static -C codegen-units=1 -O source.rs -o source.o
```

The script obtains the exported text symbol from `xcrun nm`. It passes that exact
symbol to this linker command:

```text
xcrun ld -arch arm64 -static -no_pie -platform_version macos SDK_VERSION SDK_VERSION -e ACTUAL_SYMBOL -undefined error -o tiny-arm64-static source.o
```

Measured tool facts:

- `rustc 1.98.0 (88d9e12ae 2026-08-18)`
- `ld-27037`
- macOS SDK `27.0`
- `usr/include/mach/machine.h:342` defines `CPU_SUBTYPE_ARM64_ALL` as `0`
- `usr/include/mach/machine.h:344` defines `CPU_SUBTYPE_ARM64E` as `2`
- Exported symbol `_arm64_fixture`
- SHA-256 `a55e2554b91b1a5d451df1b1125a883f9f239f610ba841caed55a75651574ac4`

The fixture test opens the artifact with `debug/macho.Open`. It passes the parsed
`FileHeader` and `ByteOrder` to `arm64.ValidateHeader`. It does not execute the
artifact or inspect its load commands.
