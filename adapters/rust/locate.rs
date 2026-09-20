// Purpose: finds a compiled function's address range by its name.
// Never:   guesses a range for a name with no compiled address.
// In:      the bytes of a binary, a function name
// Out:     the name with its start and end address
// Fails:   the file is unreadable, the name is absent, the range has no end
use object::{Object, ObjectSymbol};

#[derive(Debug, PartialEq)]
pub enum LocateError {
    Unreadable(String),
    NameAbsent(String),
    NoExtent(String),
}

#[derive(Debug, PartialEq)]
pub struct Located {
    pub name: String,
    pub start: u64,
    pub end: u64,
}

/// Rust prefixes a `#[no_mangle]` symbol with one underscore on Mach-O.
fn matches_name(symbol: &str, wanted: &str) -> bool {
    symbol == wanted || symbol.strip_prefix('_') == Some(wanted)
}

pub fn locate(content: &[u8], name: &str) -> Result<Located, LocateError> {
    let file = object::File::parse(content).map_err(|e| LocateError::Unreadable(e.to_string()))?;
    let start = named_address(&file, name).ok_or_else(|| LocateError::NameAbsent(name.to_string()))?;
    let end = next_address_after(&file, start).ok_or_else(|| LocateError::NoExtent(name.to_string()))?;
    Ok(Located { name: name.to_string(), start, end })
}

fn named_address(file: &object::File<'_>, name: &str) -> Option<u64> {
    file.symbols()
        .find(|symbol| matches!(symbol.name(), Ok(found) if matches_name(found, name)))
        .map(|symbol| symbol.address())
}

/// The next symbol's address bounds the function, because a symbol starts
/// where the one before it ends.
fn next_address_after(file: &object::File<'_>, start: u64) -> Option<u64> {
    file.symbols()
        .map(|symbol| symbol.address())
        .filter(|address| *address > start)
        .min()
}
