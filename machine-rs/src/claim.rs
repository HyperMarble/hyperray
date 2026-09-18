// What one proof asks: where the function is, and what must be true when it
// ends. A claim names registers or memory; it is never fixed by this file.
use std::fmt;

/// A place a claim can be about.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Place {
    /// A register of thread 0, by the name the model uses.
    Register(String),
    /// A named memory observation, written `*name` in the claim.
    Memory(String),
}

/// One claim about the state when the function returns.
///
/// The engine compares for equality only, but `Not` and `Any` build any
/// finite comparison from it.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Claim {
    Equals(Place, u64),
    All(Vec<Claim>),
    Any(Vec<Claim>),
    Not(Box<Claim>),
}

/// A claim that `place` holds none of `rejected`.
///
/// An ordering is stated this way because the engine has no ordering
/// operator, only equality and negation.
pub fn none_of(place: Place, rejected: impl IntoIterator<Item = u64>) -> Claim {
    let each = rejected
        .into_iter()
        .map(|value| Claim::Equals(place.clone(), value))
        .collect();
    Claim::Not(Box::new(Claim::Any(each)))
}

impl fmt::Display for Claim {
    fn fmt(&self, out: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Claim::Equals(Place::Register(name), value) => {
                write!(out, "0:{name} = 0x{value:016x}")
            }
            Claim::Equals(Place::Memory(name), value) => {
                write!(out, "*{name} = 0x{value:016x}")
            }
            Claim::All(parts) => write_joined(out, parts, " & "),
            Claim::Any(parts) => write_joined(out, parts, " | "),
            Claim::Not(inner) => write!(out, "~({inner})"),
        }
    }
}

fn write_joined(out: &mut fmt::Formatter<'_>, parts: &[Claim], between: &str) -> fmt::Result {
    write!(out, "(")?;
    for (index, part) in parts.iter().enumerate() {
        if index > 0 {
            write!(out, "{between}")?;
        }
        write!(out, "{part}")?;
    }
    write!(out, ")")
}

/// The negated form the engine is asked to refute.
///
/// A proof succeeds when the negation has no model, so the claim is wrapped
/// rather than sent directly.
pub fn negated(claim: &Claim) -> String {
    format!("~({claim})")
}
