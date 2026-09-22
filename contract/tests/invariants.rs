// Purpose: Test whole-document SMT parsing invariants.
// Never: Depend on one selected function or machine target.
// In: Mutations of one valid `.hray` file.
// Out: Rejection for every invalid mutation.
// Fails: A grammar mutation becomes accepted.

use contract::{parse, ParseError, ParseLocation};
use proptest::prelude::*;

fn source() -> String {
    "(hray (fn f) (in none) (out none) (mem none) (os none) (req true))".into()
}

fn meaning(source: &str) -> Result<Vec<(String, Vec<String>)>, ParseError> {
    parse(source).map(|contract| {
        contract
            .sections
            .into_iter()
            .map(|section| {
                let values = section.values.iter().map(ToString::to_string).collect();
                (section.name, values)
            })
            .collect()
    })
}

proptest! {
    #[test]
    fn smt_whitespace_does_not_change_meaning(spaces in 1usize..32) {
        let separator = " ".repeat(spaces);
        let changed = source().replace(' ', &separator);
        prop_assert_eq!(meaning(&changed), meaning(&source()));
    }
}

#[test]
fn trailing_terms_are_rejected() {
    let changed = format!("{} true", source());
    assert!(parse(&changed).is_err());
}

#[test]
fn trailing_term_reports_its_source_position() {
    let changed = format!("{}\ntrue", source());
    let expected = ParseLocation::Source { line: 2, column: 1 };
    assert!(matches!(parse(&changed), Err(value) if value.location == expected));
}

#[test]
fn parentheses_can_form_smt_token_boundaries() {
    let compact = "(hray(fn f)(in none)(out none)(mem none)(os none)(req true))";
    assert_eq!(meaning(compact), meaning(&source()));
}

#[test]
fn comment_at_end_of_file_keeps_the_same_meaning() {
    let commented = format!("{} ; final comment", source());
    assert_eq!(meaning(&commented), meaning(&source()));
}

#[test]
fn comment_only_error_points_to_the_real_end_of_source() {
    let source = "; comment only";
    let expected = ParseLocation::Source {
        line: 1,
        column: source.len() + 1,
    };
    assert!(matches!(parse(source), Err(value) if value.location == expected));
}
