// Purpose: Describe one exact `.hray` grammar failure.
// Never: Hide the expected or observed source text.
// In: A truthful source or document location and grammar expectation.
// Out: A stable public diagnostic value.
// Fails: This type does not perform parsing.

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ParseError {
    pub location: ParseLocation,
    pub kind: ParseErrorKind,
    pub expected: String,
    pub found: String,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum ParseLocation {
    Source { line: usize, column: usize },
    Root,
    Section { index: usize },
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum ParseErrorKind {
    InvalidUtf8,
    InvalidSmt,
    InvalidRoot,
    InvalidSection,
}

impl ParseError {
    pub(crate) fn new(
        location: ParseLocation,
        kind: ParseErrorKind,
        expected: impl Into<String>,
        found: impl Into<String>,
    ) -> Self {
        Self {
            location,
            kind,
            expected: expected.into(),
            found: found.into(),
        }
    }
}

impl std::fmt::Display for ParseError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self.location {
            ParseLocation::Source { line, column } => write!(
                formatter,
                "{line}:{column}: expected {}, found {}",
                self.expected, self.found
            ),
            ParseLocation::Root => {
                write!(
                    formatter,
                    "root: expected {}, found {}",
                    self.expected, self.found
                )
            }
            ParseLocation::Section { index } => write!(
                formatter,
                "section {index}: expected {}, found {}",
                self.expected, self.found
            ),
        }
    }
}

impl std::error::Error for ParseError {}
