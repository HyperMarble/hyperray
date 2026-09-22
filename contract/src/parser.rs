// Purpose: Decode one SMT term into `.hray` sections.
// Never: Interpret section names, values, roles, types, or machine meaning.
// In: One parsed SMT term rooted at a simple `hray` application.
// Out: Section names and their untouched SMT argument trees.
// Fails: The root or a section shape is invalid.

use crate::{syntax, Contract, Expression, ParseError, ParseErrorKind, ParseLocation, Section};
use yaspar_ir::ast::alg::Term as ExpressionKind;
pub(crate) fn parse(source: &str) -> Result<Contract, ParseError> {
    let term = syntax::parse_term(source)?;
    let (root, sections) = application(term, ParseLocation::Root, ParseErrorKind::InvalidRoot)?;
    if root != "hray" {
        return Err(structure(
            ParseLocation::Root,
            ParseErrorKind::InvalidRoot,
            "hray",
            &root,
        ));
    }
    let sections = sections
        .into_iter()
        .enumerate()
        .map(|(index, term)| parse_section(index + 1, term))
        .collect::<Result<Vec<_>, _>>()?;
    Ok(Contract { sections })
}

fn parse_section(index: usize, term: Expression) -> Result<Section, ParseError> {
    let location = ParseLocation::Section { index };
    let (name, values) = application(term, location, ParseErrorKind::InvalidSection)?;
    Ok(Section { name, values })
}

fn application(
    term: Expression,
    location: ParseLocation,
    kind: ParseErrorKind,
) -> Result<(String, Vec<Expression>), ParseError> {
    let ExpressionKind::App(identifier, arguments, _) = &**term else {
        return Err(structure(
            location,
            kind,
            "simple application",
            "other SMT term",
        ));
    };
    if !identifier.0.indices.is_empty() || identifier.1.is_some() {
        return Err(structure(
            location,
            kind,
            "simple application name",
            "qualified name",
        ));
    }
    Ok((identifier.0.symbol.as_str().to_owned(), arguments.clone()))
}

fn structure(
    location: ParseLocation,
    kind: ParseErrorKind,
    expected: &str,
    found: &str,
) -> ParseError {
    ParseError::new(location, kind, expected, found)
}
