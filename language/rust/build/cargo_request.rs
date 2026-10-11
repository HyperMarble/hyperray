// Purpose: passes complete compilation requests to Cargo's own parser.
// Never: maintains an adapter list of accepted compiler or Cargo flags.
use std::ffi::OsString;

/// Native arguments for each Cargo command used to collect the build record.
#[derive(Clone, Debug, Default, PartialEq)]
pub struct Arguments {
    pub build: Vec<OsString>,
    pub metadata: Vec<OsString>,
}

/// `--locked` for a Cargo command, unless the owner's arguments already
/// hold it: Cargo refuses the same flag given twice.
pub(crate) fn locked(given: &[OsString]) -> Option<OsString> {
    let absent = !given.iter().any(|argument| argument == "--locked");
    absent.then(|| OsString::from("--locked"))
}

/// The owner's arguments without `--message-format`. It only chooses how
/// Cargo prints, never what it builds, and the adapter needs Cargo's JSON
/// messages to learn the files.
pub(crate) fn without_message_format(given: &[OsString]) -> Vec<OsString> {
    let mut kept = Vec::new();
    let mut words = given.iter();
    while let Some(word) = words.next() {
        if word == "--message-format" {
            words.next();
        } else if !joined_message_format(word) {
            kept.push(word.clone());
        }
    }
    kept
}

fn joined_message_format(word: &OsString) -> bool {
    let text = word.to_str().unwrap_or_default();
    text.starts_with("--message-format=")
}

/// The flags that pick what a build makes or where it goes, each with
/// whether a value follows it: the ones `cargo build --help` lists under
/// "Package Selection" and "Target Selection", and `--target-dir`. A test
/// reads Cargo's own help against this list.
pub const PICKS: &[(&str, bool)] = &[
    ("--package", true),
    ("-p", true),
    ("--workspace", false),
    ("--exclude", true),
    ("--all", false),
    ("--lib", false),
    ("--bins", false),
    ("--bin", true),
    ("--examples", false),
    ("--example", true),
    ("--tests", false),
    ("--test", true),
    ("--benches", false),
    ("--bench", true),
    ("--all-targets", false),
    ("--target-dir", true),
];

/// The owner's arguments without the picking flags and their values, for a
/// request that names one library and its own folder itself.
pub(crate) fn without_picks(given: &[OsString]) -> Vec<OsString> {
    let mut kept = Vec::new();
    let mut words = given.iter().peekable();
    while let Some(word) = words.next() {
        let text = word.to_str().unwrap_or_default();
        match PICKS.iter().find(|(flag, _)| text == *flag) {
            Some((_, true)) => drop(words.next_if(|next| !next.to_string_lossy().starts_with('-'))),
            Some(_) => {}
            None if joined_pick(text) => {}
            None => kept.push(word.clone()),
        }
    }
    kept
}

/// True for a picking flag written with its value attached, as in
/// `--bin=name` or `-pname`.
fn joined_pick(text: &str) -> bool {
    let attached = |flag: &str| text.starts_with(&format!("{flag}="));
    PICKS.iter().any(|(flag, _)| attached(flag))
        || (text.starts_with("-p") && !text.starts_with("-p="))
}
