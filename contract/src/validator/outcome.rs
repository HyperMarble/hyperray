// Purpose: Report an exact contract rejection or validator engine error.
// Never: Turn incomplete upstream facts into a program verdict.
// In: A failed contract condition or an incomplete validator dependency.
// Out: Public rejection or engine-error data, plus validated contracts.
// Fails: These values do not perform validation.

use crate::Contract;

use super::facts::{FunctionFact, OsOperationFact};

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ValidationIssue {
    pub section_index: Option<usize>,
    pub message: String,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum ValidationFailure {
    Rejected(ValidationIssue),
    Engine(ValidationIssue),
}

impl ValidationFailure {
    pub(crate) fn rejected(index: Option<usize>, message: impl Into<String>) -> Self {
        Self::Rejected(ValidationIssue {
            section_index: index,
            message: message.into(),
        })
    }

    pub(crate) fn engine(index: Option<usize>, message: impl Into<String>) -> Self {
        Self::Engine(ValidationIssue {
            section_index: index,
            message: message.into(),
        })
    }

    fn issue(&self) -> &ValidationIssue {
        match self {
            Self::Rejected(issue) | Self::Engine(issue) => issue,
        }
    }
}

impl std::fmt::Display for ValidationFailure {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let issue = self.issue();
        match issue.section_index {
            Some(index) => write!(formatter, "section {index}: {}", issue.message),
            None => issue.message.fmt(formatter),
        }
    }
}

impl std::error::Error for ValidationFailure {}

#[derive(Clone, Debug)]
pub struct ValidatedContract {
    pub contract: Contract,
    pub function: FunctionFact,
    pub os_operations: Vec<OsOperationFact>,
}
