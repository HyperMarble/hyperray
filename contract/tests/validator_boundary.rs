// Purpose: Keep semantic checks outside the `.hray` syntax parser.
// Never: Require compiler, model, ABI, or interface metadata to parse text.
// In: Syntactically valid names and expressions with unknown meaning.
// Out: Parsed contracts for the later validator.
// Fails: The parser starts enforcing validator policy.

use contract::parse;

#[test]
fn accepts_names_that_only_metadata_can_resolve() {
    let source = "(hray (fn unknown_function) (in none) (out none) (mem none) \
        (os unknown_interface unknown_operation) (req unknown_role))";
    assert!(parse(source).is_ok());
}

#[test]
fn accepts_role_use_that_only_type_validation_can_reject() {
    let source = "(hray (fn f) (in arg999999999999999999999999 all) \
        (in when ret) (out none) (mem none) (os none) (req memory_after))";
    assert!(parse(source).is_ok());
}

#[test]
fn accepts_operation_arity_that_only_registry_validation_can_reject() {
    let source = "(hray (fn f) (in none) (out none) (mem none) (os none) \
        (req (unknown_operation true false true)))";
    assert!(parse(source).is_ok());
}
