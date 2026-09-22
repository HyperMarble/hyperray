// Purpose: Parse one complete source value with the official SMT parser.
// Never: Interpret the `.hray` root, sections, roles, or machine meaning.
// In: UTF-8 text that must contain exactly one SMT term.
// Out: The official SMT parser's term tree and source ranges.
// Fails: SMT syntax is invalid, empty, or followed by more source.

use crate::{Expression, ParseError, ParseErrorKind, ParseLocation};
use lalrpop_util::ParseError as LalrpopError;
use yaspar::ast::GrammarError;
use yaspar::position::{Position, Range};
use yaspar::tokens::Token;
use yaspar_ir::untyped::UntypedAst;

type SmtError = LalrpopError<Position, Token, GrammarError>;

pub(crate) fn parse_term(source: &str) -> Result<Expression, ParseError> {
    UntypedAst
        .parse_term_str(source)
        .map_err(|error| map_error(source, error))
}

fn map_error(source: &str, error: SmtError) -> ParseError {
    let position = error_position(source, &error);
    invalid(position, error.to_string())
}

fn error_position(source: &str, error: &SmtError) -> Position {
    match error {
        LalrpopError::InvalidToken { location } => *location,
        LalrpopError::UnrecognizedEof { .. } => source_end(source),
        LalrpopError::UnrecognizedToken { token, .. } | LalrpopError::ExtraToken { token } => {
            token.0
        }
        LalrpopError::User { error } => grammar_range(error).start,
    }
}

fn grammar_range(error: &GrammarError) -> &Range {
    match error {
        GrammarError::TokenizeError { range, .. }
        | GrammarError::DatatypeDeclarationError { range, .. }
        | GrammarError::RecFunsDefinitionError { range, .. }
        | GrammarError::Other { range, .. } => range,
    }
}

fn source_end(source: &str) -> Position {
    source
        .chars()
        .fold(Position::default(), |mut position, character| {
            position.incr(character);
            position
        })
}

fn invalid(position: Position, found: impl Into<String>) -> ParseError {
    ParseError::new(
        ParseLocation::Source {
            line: position.lin_num + 1,
            column: position.col_num + 1,
        },
        ParseErrorKind::InvalidSmt,
        "one SMT term",
        found,
    )
}
