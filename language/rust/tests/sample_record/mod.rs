// Purpose: one complete, made-up build record for the JSON shape checks.
// Never:   stands in for a real build; real builds are checked elsewhere.
use language_rust::build_facts::Compiled;
use language_rust::record::{Artifact, BuildRecord, FileDigest, Settings, Toolchain};

pub fn digest(path: &str) -> FileDigest {
    FileDigest {
        path: path.to_string(),
        sha256: "ab".repeat(32),
    }
}

fn artifact() -> Artifact {
    let compiled = Compiled {
        opt_level: "3".to_string(),
        debuginfo: "0".to_string(),
        debug_assertions: false,
        overflow_checks: true,
        test: false,
    };
    let features = vec!["fancy".to_string()];
    let file = digest("libsample.rlib");
    Artifact {
        kind: "lib".to_string(),
        file,
        features,
        compiled,
        debug_info: None,
    }
}

pub fn record() -> BuildRecord {
    let toolchain = Toolchain {
        version: "1.98.1".to_string(),
        host: "aarch64-apple-darwin".to_string(),
        compiler: digest("rustc"),
    };
    let settings = Settings {
        profile: "release".to_string(),
        lock_file: digest("Cargo.lock"),
    };
    BuildRecord {
        language: "rust".to_string(),
        artifacts: vec![artifact()],
        toolchain,
        settings,
        os_build: "macOS 27.0 (25A123)".to_string(),
    }
}
