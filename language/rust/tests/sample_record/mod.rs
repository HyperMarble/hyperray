// Purpose: one complete, made-up build record for the JSON shape checks.
// Never:   stands in for a real build; real builds are checked elsewhere.
use sha2::Digest;

use language_rust::build_facts::{
    CToolchain,
    Compiled,
    EnvVar,
    NativeCode, //
};
use language_rust::record::{
    Artifact,
    BuildRecord,
    FileDigest,
    Settings,
    Toolchain, //
};

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
    Artifact {
        kind: "lib".to_string(),
        file: digest("libsample.rlib"),
        features: vec!["fancy".to_string()],
        compiled,
        debug_info: None,
    }
}

fn c_toolchain() -> CToolchain {
    CToolchain {
        compiler: digest("clang"),
        version: "Apple clang version 27.0.0".to_string(),
        sdk_path: None,
        sdk_version: Some("27.0".to_string()),
    }
}

pub fn record() -> BuildRecord {
    let native = NativeCode {
        package: "zstd-sys".to_string(),
        linked_libs: vec!["static=zstd".to_string()],
    };
    BuildRecord {
        language: "rust".to_string(),
        cargo_request: None,
        artifacts: vec![artifact()],
        toolchain: Toolchain {
            version: "1.98.1".to_string(),
            host: "aarch64-apple-darwin".to_string(),
            compiler: digest("rustc"),
        },
        settings: Settings {
            lock_file: digest("Cargo.lock"),
            config_files: vec![digest(".cargo/config.toml")],
            missing_optional_config_files: Vec::new(),
        },
        environment: vec![EnvVar {
            name: Some("RUSTFLAGS".to_string()),
            name_encoded: b"RUSTFLAGS".to_vec(),
            value: None,
            value_sha256: format!("{:x}", sha2::Sha256::digest("-C target-cpu=native")),
        }],
        native_code: vec![native],
        c_toolchain: c_toolchain(),
        os_build: "macOS 27.0 (25A123)".to_string(),
    }
}
