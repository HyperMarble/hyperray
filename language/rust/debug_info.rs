// Purpose: finds the debug information inside Apple's `.dSYM` bundle, which
//          is a folder, and names it by the hash of its bytes.
// Never:   guesses which file holds it: the bundle's `Contents/Resources/DWARF`
//          folder must hold exactly one file, or the build is blocked.
use crate::blocked::Blocked;
use crate::digest::digest_of;
use crate::record::FileDigest;
use std::path::Path;

/// The debug-info file inside `bundle`, with its hash.
pub fn dwarf_digest(bundle: &Path) -> Result<FileDigest, Blocked> {
    let folder = bundle.join("Contents").join("Resources").join("DWARF");
    let unreadable = |cause: String| Blocked::Unreadable {
        what: folder.display().to_string(),
        cause,
    };
    let entries = std::fs::read_dir(&folder).map_err(|e| unreadable(e.to_string()))?;
    let files: Vec<_> = entries
        .filter_map(|entry| entry.ok().map(|entry| entry.path()))
        .filter(|path| path.is_file())
        .collect();
    let [file] = files.as_slice() else {
        return Err(unreadable(format!(
            "expected one debug-info file, found {}",
            files.len()
        )));
    };
    digest_of(file)
}
