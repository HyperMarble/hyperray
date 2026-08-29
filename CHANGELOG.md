# Changelog

## v0.1.1

- Installer shows live progress steps and a ready banner
- New `ray update` command: self-update to the latest release
- New `ray version` command
- CI workflow: build, vet, and the full test suite on every push
- Dead command files removed; comments trimmed to plain speech

## v0.1.0

- First release: the fail-closed authoring ladder (`ray verify`) for one frozen bounded task
- Python, Rust, and C++ support across all rungs, proven on approved tasks
- Probes generated per language; author writes one entry per operation
- Statement gate (ASCII, word budget, promise coverage), repo lint gate, bundle gate
- Release binaries for darwin/linux on arm64/amd64, curl installer, brew tap
