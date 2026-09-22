// Purpose: Generate complete contracts from every fixed `.hray` section form.
// Never: Use a selected fixture as evidence for the complete document grammar.
// In: Generated symbols, section alternatives, whitespace, comments, and repetitions.
// Out: Acceptance for valid documents and exact rejection for a missing root close.
// Fails: Document structure depends on one spelling or whitespace layout.

use contract::{parse, ParseLocation};
use proptest::prelude::*;

fn contract() -> impl Strategy<Value = String> {
    let name = "hray_[A-Za-z][A-Za-z0-9_]{0,12}";
    let input = prop::sample::select(vec![
        "(in none)",
        "(in arg1 all)",
        "(in arg1 all) (in when true)",
    ]);
    let output = prop::sample::select(vec!["(out none)", "(out ret)"]);
    let memory = prop::sample::select(vec![
        "(mem none)",
        "(mem arg1 read #x08 bytes)",
        "(mem arg1 write #x08 bytes) (mem arg2 read #x04 bytes)",
    ]);
    let os = prop::sample::select(vec!["(os none)", "(os wasi_snapshot_preview1 fd_read)"]);
    let separator = prop::sample::select(vec![" ", "\n", "\t", " ; generated\n"]);
    (name, input, output, memory, os, separator).prop_map(
        |(name, input, output, memory, os, separator)| {
            let sections = [
                format!("(fn {name})"),
                input.into(),
                output.into(),
                memory.into(),
                os.into(),
                "(req true)".into(),
            ];
            format!("(hray{separator}{}{separator})", sections.join(separator))
        },
    )
}

fn source_end(source: &str) -> ParseLocation {
    let (line, column) = source.chars().fold((1, 1), |position, character| {
        if character == '\n' {
            return (position.0 + 1, 1);
        }
        (position.0, position.1 + 1)
    });
    ParseLocation::Source { line, column }
}

proptest! {
    #[test]
    fn accepts_every_generated_section_form(source in contract()) {
        prop_assert!(parse(&source).is_ok(), "rejected: {source}");
    }

    #[test]
    fn missing_root_close_fails_at_source_end(mut source in contract()) {
        prop_assert_eq!(source.pop(), Some(')'));
        let expected = source_end(&source);
        prop_assert!(matches!(parse(&source), Err(value) if value.location == expected));
    }
}
