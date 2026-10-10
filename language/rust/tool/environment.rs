// Purpose: records the environment variables that can change a Rust build:
//          the ones Cargo, rustc and rustup read, and the ones the C compiler
//          helper (the `cc` crate) reads for C code built into the program.
// Never:   writes a secret's value: a name that looks like a credential is
//          recorded as set, with its value hidden.
use crate::build_facts::EnvVar;

/// Exact names: the `cc` crate's own list, plus Apple's SDK settings.
const NAMES: [&str; 17] = [
    "CC",
    "CFLAGS",
    "CXX",
    "CXXFLAGS",
    "AR",
    "ARFLAGS",
    "CXXSTDLIB",
    "CXXSTDLIB_STATIC",
    "CRATE_CC_NO_DEFAULTS",
    "CC_SHELL_ESCAPED_FLAGS",
    "CC_FORCE_DISABLE",
    "SDKROOT",
    "MACOSX_DEPLOYMENT_TARGET",
    "DEVELOPER_DIR",
    "RUSTFLAGS",
    "RUSTC",
    "RUSTC_WRAPPER",
];

/// Prefixes: Cargo's settings and Rust's tools, and the `cc` crate's
/// target-specific and `HOST_` / `TARGET_` forms.
const PREFIXES: [&str; 9] = [
    "CARGO", "RUSTC_", "RUSTDOC", "RUSTUP_", "CC_", "CFLAGS_", "CXX_", "HOST_", "TARGET_",
];

/// Words that mark a variable as a secret.
const SECRET_WORDS: [&str; 5] = ["TOKEN", "SECRET", "PASSWORD", "CREDENTIAL", "KEY"];

/// The build-relevant variables among `vars`, sorted by name.
pub fn build_environment(vars: impl Iterator<Item = (String, String)>) -> Vec<EnvVar> {
    let mut found: Vec<EnvVar> = vars
        .filter(|(name, _)| matters(name))
        .map(|(name, value)| EnvVar {
            value: (!is_secret(&name)).then_some(value),
            name,
        })
        .collect();
    found.sort_by(|a, b| a.name.cmp(&b.name));
    found
}

fn matters(name: &str) -> bool {
    NAMES.contains(&name) || PREFIXES.iter().any(|prefix| name.starts_with(prefix))
}

fn is_secret(name: &str) -> bool {
    SECRET_WORDS.iter().any(|word| name.contains(word))
}
