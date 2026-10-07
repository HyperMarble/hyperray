// Purpose: the build-relevant environment is recorded, everything else is
//          left out, and a secret's value never reaches the record.
// Never:   writes the value of a variable whose name marks it as a secret.
use language_rust::environment::build_environment;

fn var(name: &str, value: &str) -> (String, String) {
    (name.to_string(), value.to_string())
}

#[test]
fn build_variables_are_kept_and_secrets_hidden() {
    let vars = [
        var("HOME", "/Users/someone"),
        var("RUSTFLAGS", "-C target-cpu=native"),
        var("CC", "clang"),
        var("CFLAGS_aarch64_apple_darwin", "-O2"),
        var("CARGO_REGISTRY_TOKEN", "do-not-record"),
        var("PATH", "/usr/bin"),
    ];
    let found = build_environment(vars.into_iter());
    let names: Vec<&str> = found.iter().map(|v| v.name.as_str()).collect();
    assert_eq!(
        names,
        [
            "CARGO_REGISTRY_TOKEN",
            "CC",
            "CFLAGS_aarch64_apple_darwin",
            "RUSTFLAGS"
        ]
    );
    assert_eq!(found[0].value, None, "a secret's value was recorded");
    assert_eq!(found[3].value.as_deref(), Some("-C target-cpu=native"));
}
