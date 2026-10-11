// Purpose: an owner's own `--message-format` does not stop the build.
// Never:   lets the way Cargo prints decide whether the files are recorded.
mod sample_project;

use language_rust::cargo_build::build;
use language_rust::cargo_request::Arguments;
use language_rust::project::find;
use sample_project::{
    create,
    Sample, //
};

fn built_with(name: &str, words: &[&str]) -> Result<bool, String> {
    let root = create(&Sample {
        name,
        file: "lib.rs",
        source: "pub fn one() -> u8 { 1 }\n",
        locked: true,
    })?;
    let project = find(&root).map_err(|blocked| blocked.to_string())?;
    let supplied = Arguments {
        build: words.iter().map(Into::into).collect(),
        ..Arguments::default()
    };
    let output = build(&project, &supplied).map_err(|blocked| blocked.to_string())?;
    Ok(!output.files.is_empty())
}

#[test]
fn an_owners_message_format_does_not_stop_the_build() -> Result<(), String> {
    assert!(built_with(
        "format_two_words",
        &["--message-format", "short"]
    )?);
    Ok(())
}

#[test]
fn an_owners_joined_message_format_does_not_stop_the_build() -> Result<(), String> {
    assert!(built_with("format_one_word", &["--message-format=short"])?);
    Ok(())
}
