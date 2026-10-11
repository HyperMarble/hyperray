# Rust adapter

Builds a Cargo project the project's own way, records every file the build
made and what made it that way, and hands the files to the loader.

## What it does

1. Runs the project's own build: `cargo build` with the owner's flags as
   the owner wrote them. It adds `--locked`, so the build uses the versions
   the lock file pins, and asks Cargo for its JSON messages.
2. Picks up the machine code: every file Cargo reports for the project, and
   every file one of those loads at run time.
3. Names each file by the hash of its bytes, and notes the compiler and the
   settings that made it.
4. Hands one record to the loader. When there is no build, the record holds
   the reason, in Cargo's own words when Cargo gave it.

It never looks inside the code. The compiler turns every Rust shape into
machine code, and the adapter picks that machine code up.

## Added

- [x] Every kind of file Cargo builds: program, `lib`, `rlib`, `dylib`,
      `cdylib`, `staticlib`, proc macro, build script. A file carries every
      kind Cargo reports for it.
- [x] Every flag `cargo build --help` names, given to Cargo as written.
      `--message-format` is the one left out: the adapter needs the JSON.
- [x] A workspace member, built with the workspace's lock file.
- [x] A crate the project depends on by path, outside its workspace.
- [x] The finished form of each library. An `.rlib` still has places the
      linker has not filled, so Cargo is asked for the linked one.
- [x] A library without the standard library, finished through a small
      crate of the adapter's own.
- [x] A dylib crate from outside the project. The program loads it at run
      time, so its code is in no other file.
- [x] A library the compiler ships that a built file loads at run time,
      such as Rust's standard library as a shared file.
- [x] Test programs, when the owner asks for every target (`--all-targets`).
- [x] Each file's debug-info file, when the build made one.
- [x] The compiler Cargo really runs, also when the project pins one or
      the owner names another with `RUSTC` or `build.rustc`, and the chip
      `--target` names.
- [x] The lock file and every Cargo settings file that shaped the build,
      with the files they include.
- [x] Every environment variable, by its name and the hash of its value.
- [x] C code a build script links in, with the C compiler and SDK used.
- [x] The OS build.
- [x] A build Cargo refuses, handed back with Cargo's own words.

## Left

- [ ] `--manifest-path` naming another project. Cargo builds it, the
      adapter refuses it.
- [ ] `--features` naming a feature one crate of the project lacks. That
      crate's finished library is not made, and the record says so.
- [ ] A library without the standard library in a project that sets its own
      profile, patch or replace. Its finished library is not made, and the
      record says so.
- [ ] Linux. Not run yet.
