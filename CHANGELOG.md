# Changelog

## v0.1.2

- Renamed to hyperray (binary, module, installer, tap); config file stays hyperray.toml for now

## v0.1.1

- Installer shows live progress steps and a ready banner
- New `hyperray update` command: self-update to the latest release
- New `hyperray version` command
- CI workflow: build, vet, and the full test suite on every push
- Dead command files removed; comments trimmed to plain speech

## v0.1.0

- First release: the fail-closed authoring ladder (`hyperray verify`) for one frozen bounded task
- Python, Rust, and C++ support across all rungs, proven on approved tasks
- Probes generated per language; author writes one entry per operation
- Statement gate (ASCII, word budget, promise coverage), repo lint gate, bundle gate
- Release binaries for darwin/linux on arm64/amd64, curl installer, brew tap
