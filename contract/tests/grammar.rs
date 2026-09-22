// Purpose: Test the small structure that the `.hray` parser owns.
// Never: Enforce section order, counts, contents, or machine meaning.
// In: SMT roots and direct child terms.
// Out: One `hray` application containing named section applications.
// Fails: A wrong root or non-section child reaches the validator.

use contract::{parse, ParseLocation};

#[test]
fn rejects_wrong_root_and_non_section_children() {
    let cases = ["(other (fn f))", "(hray true)", "hray"];
    assert!(cases.iter().all(|source| parse(source).is_err()));
}

#[test]
fn leaves_section_policy_for_the_validator() {
    let source = "(hray (req true) (unknown none) (fn f) (in none) (in arg1 all))";
    assert!(parse(source).is_ok());
}

#[test]
fn reports_document_locations_without_invented_source_positions() {
    let root = parse("(other (fn f))");
    let section = parse("(hray true)");
    assert!(matches!(root, Err(value) if value.location == ParseLocation::Root));
    assert!(matches!(
        section,
        Err(value) if value.location == ParseLocation::Section { index: 1 }
    ));
}
