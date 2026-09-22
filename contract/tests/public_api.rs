// Purpose: Test the parser through its public interface.
// Never: Inspect private parser state.
// In: Complete `.hray` byte and text inputs.
// Out: Observable contracts or positioned errors.
// Fails: The public parser accepts invalid source or hides valid source.

use contract::{parse, parse_bytes, MetaData, ParseLocation};

const MINIMAL: &str = "(hray (fn advance) (in none) (out none) (mem none) (os none) (req true))";

#[test]
fn parses_minimal_contract() {
    let contract = parse(MINIMAL);
    let names = contract.map(|value| {
        value
            .sections
            .into_iter()
            .map(|section| section.name)
            .collect::<Vec<_>>()
    });
    let expected = ["fn", "in", "out", "mem", "os", "req"]
        .map(String::from)
        .to_vec();
    assert_eq!(names, Ok(expected));
}

#[test]
fn parses_quoted_names_and_nested_memory() {
    let source = "(hray (fn |name with space|) (in none) (out none) \
        (mem (bvadd #x00 #x01) read (bvadd #x00 #x01) bytes) \
        (os |os interface| |operation name|) (req true))";
    let contract = parse(source);
    assert!(contract.is_ok());
}

#[test]
fn reports_invalid_utf8_from_bytes() {
    let source = [b'h', b'\n', b'a', 0xff];
    let error = parse_bytes(&source);
    let expected = ParseLocation::Source { line: 2, column: 2 };
    assert!(matches!(error, Err(value) if value.location == expected));
}

#[test]
fn returned_expressions_are_observable() {
    let contract = parse(MINIMAL);
    let section = contract
        .as_ref()
        .ok()
        .and_then(|value| value.sections.first());
    let expression = section.and_then(|value| value.values.first());
    let observed = expression.map(|value| {
        let start = value.meta_data().start;
        (value.to_string(), start.lin_num, start.col_num)
    });
    let expected = MINIMAL
        .find("advance")
        .map(|column| ("advance".into(), 0, column));
    assert!(contract.is_ok());
    assert_eq!(observed, expected);
}
