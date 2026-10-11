// Purpose: every environment input is represented by a value hash.
// Never:   copies any inherited environment value into the record.
use language_rust::environment::native_environment;
use std::ffi::OsString;

use sha2::{
    Digest,
    Sha256, //
};

fn var(name: &str, value: &str) -> (OsString, OsString) {
    (OsString::from(name), OsString::from(value))
}

#[test]
fn every_environment_value_is_hashed_and_hidden() {
    let vars = [
        var("HOME", "/Users/someone"),
        var("RUSTFLAGS", "-C target-cpu=native"),
        var("CC", "clang"),
        var("CFLAGS_aarch64_apple_darwin", "-O2"),
        var("CARGO_REGISTRY_TOKEN", "do-not-record"),
        var("PATH", "/usr/bin"),
    ];
    let found = native_environment(vars.into_iter());
    let names: Vec<Option<&str>> = found.iter().map(|v| v.name.as_deref()).collect();
    assert_eq!(
        names,
        [
            Some("CARGO_REGISTRY_TOKEN"),
            Some("CC"),
            Some("CFLAGS_aarch64_apple_darwin"),
            Some("HOME"),
            Some("PATH"),
            Some("RUSTFLAGS")
        ]
    );
    assert!(found.iter().all(|item| item.value.is_none()));
    assert_eq!(
        found[0].value_sha256,
        format!("{:x}", Sha256::digest("do-not-record"))
    );
    assert_eq!(
        found[5].value_sha256,
        format!("{:x}", Sha256::digest("-C target-cpu=native"))
    );
}
