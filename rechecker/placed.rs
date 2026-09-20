// Purpose: asks the operating system to load a built library and find a function.
// Never:   copies instructions out of the program they were linked into.
// In:      the path of a built library, and a function's name
// Out:     the address the operating system placed that function at
// Fails:   the library will not load, or it defines no such name
use crate::witness::RecheckError;
use libloading::{Library, Symbol};

/// A library the operating system has placed in memory.
///
/// The operating system places code and its data together, so an
/// instruction that counts from itself to reach a value still reaches it.
/// Copying the instructions elsewhere breaks that count.
pub struct Placed {
    library: Library,
}

impl Placed {
    /// Loads a built library the way the operating system runs any program.
    pub fn load(path: &std::path::Path) -> Result<Self, RecheckError> {
        // Safety: loading runs the library's own start-up code, the same
        // code that runs when a program uses it normally.
        let library = unsafe { Library::new(path) }
            .map_err(|error| RecheckError::Unavailable(error.to_string()))?;
        Ok(Placed { library })
    }

    /// Returns where the operating system placed one named function.
    pub fn address_of(&self, name: &str) -> Result<*const u8, RecheckError> {
        // Safety: the address is read, not called, so its type is opaque.
        // Taking it out of the symbol keeps it valid while the library is
        // loaded, which it is for as long as this value exists.
        unsafe {
            let symbol: Symbol<'_, *const u8> = self
                .library
                .get(name.as_bytes())
                .map_err(|_| RecheckError::MissingRegister(name.to_string()))?;
            Ok(symbol.into_raw().into_raw() as *const u8)
        }
    }
}
