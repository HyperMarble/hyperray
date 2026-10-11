// Purpose: picks out the owner's flags that choose Cargo's settings or the
//          chip to build for. A request for a crate of the adapter's own is
//          given these and none of the owner's other flags.
// Never:   changes a flag or its value.
use std::ffi::OsString;

/// The flags, as `cargo build --help` names them. Each one takes a value.
pub const SETTINGS: [&str; 2] = ["--config", "--target"];

/// The owner's settings flags, each with its value. A flag is written as
/// two words, `--target x`, or as one, `--target=x`.
pub fn settings(given: &[OsString]) -> Vec<OsString> {
    let mut kept = Vec::new();
    let mut words = given.iter();
    while let Some(word) = words.next() {
        if is_setting_with_value(word) {
            kept.push(word.clone());
        }
        if is_setting(word) {
            kept.push(word.clone());
            // The flag's value is the next word.
            kept.extend(words.next().cloned());
        }
    }
    kept
}

/// True for a settings flag standing alone: its value is the next word.
fn is_setting(word: &OsString) -> bool {
    SETTINGS.iter().any(|flag| word == *flag)
}

/// True for a settings flag written with its value, as in `--target=x`.
fn is_setting_with_value(word: &OsString) -> bool {
    let text = word.to_string_lossy();
    SETTINGS
        .iter()
        .any(|flag| text.starts_with(&format!("{flag}=")))
}
