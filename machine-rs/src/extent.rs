// Where one function starts and ends, taken from the binary's own symbol and
// section tables. It must not guess a boundary the file does not state.
use object::{Object, ObjectSection, ObjectSymbol};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Extent {
    pub start: u64,
    pub end: u64,
}

/// The address range of a named function.
///
/// The end is the next symbol above the start, or the containing section's
/// end when no symbol follows. Mach-O records size 0 for every symbol, so the
/// symbol's own size cannot supply it.
pub fn of(binary: &[u8], name: &str) -> Result<Extent, String> {
    let file = object::File::parse(binary).map_err(|error| error.to_string())?;
    let start = named(&file, name)?;
    let end = next_symbol_above(&file, start)
        .or_else(|| containing_section_end(&file, start))
        .ok_or_else(|| format!("{name} lies in no section, so its end is not stated"))?;
    Ok(Extent { start, end })
}

fn next_symbol_above(file: &object::File<'_>, start: u64) -> Option<u64> {
    file.symbols()
        .filter(|symbol| symbol.is_definition())
        .map(|symbol| symbol.address())
        .filter(|address| *address > start)
        .min()
}

fn containing_section_end(file: &object::File<'_>, start: u64) -> Option<u64> {
    file.sections()
        .map(|section| (section.address(), section.address() + section.size()))
        .find(|(low, high)| *low <= start && start < *high)
        .map(|(_, high)| high)
}

/// The address of one symbol, with or without the Mach-O leading underscore.
fn named(file: &object::File<'_>, name: &str) -> Result<u64, String> {
    let underscored = format!("_{name}");
    for symbol in file.symbols() {
        let Ok(found) = symbol.name() else {
            continue;
        };
        if found == name || found == underscored {
            return Ok(symbol.address());
        }
    }
    Err(format!("no symbol named {name}"))
}
