// Purpose: what the adapter does with every flag `cargo build --help` names.
// Never:   holds a flag the adapter refuses for being a mode: it refuses none.

/// Given to Cargo exactly as the owner wrote it.
pub const AS_GIVEN: &str = "given to Cargo as the owner wrote it";

/// Given to Cargo as written; stable Cargo refuses it in its own words.
pub const UNSTABLE: &str = "given to Cargo as written; stable Cargo refuses it";

/// Every flag by its long name, or its only name, with what happens to it.
pub const HANDLING: &[(&str, &str)] = &[
    ("--all", AS_GIVEN),
    ("--all-features", AS_GIVEN),
    ("--all-targets", AS_GIVEN),
    ("--artifact-dir", UNSTABLE),
    ("--bench", AS_GIVEN),
    ("--benches", AS_GIVEN),
    ("--bin", AS_GIVEN),
    ("--bins", AS_GIVEN),
    ("--color", AS_GIVEN),
    (
        "--config",
        "given to Cargo, and every config file it names is hashed",
    ),
    ("--example", AS_GIVEN),
    ("--examples", AS_GIVEN),
    ("--exclude", AS_GIVEN),
    ("--features", AS_GIVEN),
    ("--frozen", AS_GIVEN),
    ("--future-incompat-report", AS_GIVEN),
    (
        "--help",
        "not a build: Cargo prints its help and makes no files",
    ),
    ("--ignore-rust-version", AS_GIVEN),
    ("--jobs", AS_GIVEN),
    ("--keep-going", AS_GIVEN),
    ("--lib", AS_GIVEN),
    (
        "--locked",
        "always asked for, and never given to Cargo twice",
    ),
    ("--manifest-path", AS_GIVEN),
    (
        "--message-format",
        "left out: the adapter needs Cargo's JSON messages",
    ),
    ("--no-default-features", AS_GIVEN),
    ("--offline", AS_GIVEN),
    ("--package", AS_GIVEN),
    ("--profile", AS_GIVEN),
    ("--quiet", AS_GIVEN),
    ("--release", AS_GIVEN),
    ("--target", AS_GIVEN),
    ("--target-dir", AS_GIVEN),
    ("--test", AS_GIVEN),
    ("--tests", AS_GIVEN),
    ("--timings", AS_GIVEN),
    ("--unit-graph", UNSTABLE),
    ("--verbose", AS_GIVEN),
    ("--workspace", AS_GIVEN),
    ("-Z", UNSTABLE),
];
