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
