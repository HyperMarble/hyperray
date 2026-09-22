// Purpose: Expose the SMT-shaped `.hray` parser and its public values.
// Never: Validate section meaning, compiler facts, or machine behavior.
// In: One UTF-8 SMT term rooted at `hray`.
// Out: Untouched section terms or a positioned parse error.
// Fails: Source is not one structurally valid `.hray` SMT term.

mod ast;
mod error;
mod parser;
mod syntax;
mod validator;

pub use ast::{Contract, Section};
pub use error::{ParseError, ParseErrorKind, ParseLocation};
pub use validator::*;
pub use yaspar_ir::traits::MetaData;
pub use yaspar_ir::untyped::Term as Expression;

pub fn parse(source: &str) -> Result<Contract, ParseError> {
    parser::parse(source)
}

pub fn parse_bytes(source: &[u8]) -> Result<Contract, ParseError> {
    match std::str::from_utf8(source) {
        Ok(text) => parse(text),
        Err(error) => Err(invalid_utf8(source, error.valid_up_to())),
    }
}

fn invalid_utf8(source: &[u8], offset: usize) -> ParseError {
    let prefix = &source[..offset];
    let line = prefix.iter().filter(|byte| **byte == b'\n').count() + 1;
    let column = prefix
        .iter()
        .rev()
        .take_while(|byte| **byte != b'\n')
        .count()
        + 1;
    ParseError::new(
        ParseLocation::Source { line, column },
        ParseErrorKind::InvalidUtf8,
        "UTF-8 source",
        "invalid byte",
    )
}
