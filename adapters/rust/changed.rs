// Purpose: reports which functions have different instructions after a change.
// Never:   passes over a function that exists in only one of the two builds.
// In:      two builds, and the names to compare
// Out:     the functions whose instructions differ, with both versions
// Fails:   a build is unreadable, or a named function has no extent
use crate::locate::{locate, Located, LocateError};

#[derive(Debug, PartialEq)]
pub enum Difference {
    /// The instructions differ, so the function needs proving again.
    Changed { name: String, before: Vec<u8>, after: Vec<u8> },
    /// The function exists in only one of the two builds.
    OnlyInOne { name: String, present_before: bool },
}

/// Returns the functions whose emitted instructions are not identical.
///
/// Source text can change without changing the instructions, as `n / 2` and
/// `n >> 1` both compile to one shift. Comparing instructions rather than
/// source avoids proving a function whose compiled form did not move.
pub fn differing(before: &[u8], after: &[u8], names: &[String]) -> Result<Vec<Difference>, LocateError> {
    let mut differences = Vec::new();
    for name in names {
        let old = instructions(before, name)?;
        let new = instructions(after, name)?;
        match (old, new) {
            (Some(old), Some(new)) if old != new => {
                differences.push(Difference::Changed { name: name.clone(), before: old, after: new })
            }
            (Some(_), None) => {
                differences.push(Difference::OnlyInOne { name: name.clone(), present_before: true })
            }
            (None, Some(_)) => {
                differences.push(Difference::OnlyInOne { name: name.clone(), present_before: false })
            }
            _ => continue,
        }
    }
    Ok(differences)
}

/// Reads a function's bytes, or reports that the build does not define it.
fn instructions(content: &[u8], name: &str) -> Result<Option<Vec<u8>>, LocateError> {
    let placed = match locate(content, name) {
        Ok(placed) => placed,
        Err(LocateError::NameAbsent(_)) => return Ok(None),
        Err(other) => return Err(other),
    };
    Ok(Some(bytes_at(content, &placed)))
}

fn bytes_at(content: &[u8], placed: &Located) -> Vec<u8> {
    let start = placed.start as usize;
    let end = (placed.end as usize).min(content.len());
    if start >= end {
        return Vec::new();
    }
    content[start..end].to_vec()
}
