// Where the tests find what lives outside the repository. Patches are
// under HYPERRAY_FIXTURES, source trees under HYPERRAY_FIXTURE_SRC, the
// Charon binary at HYPERRAY_CHARON. Unset means the test skips, aloud.

use std::path::PathBuf;

pub fn dir(name: &str) -> Option<PathBuf> {
    let path = PathBuf::from(std::env::var(name).ok()?);
    if !path.exists() {
        eprintln!("skipped: {name} is unset or does not exist");
        return None;
    }
    Some(path)
}

// Every `<fixture>/solution.patch` under HYPERRAY_FIXTURES, sorted.
#[allow(dead_code)]
pub fn patches() -> Vec<(String, String)> {
    let Some(root) = dir("HYPERRAY_FIXTURES") else {
        return Vec::new();
    };
    let Ok(entries) = std::fs::read_dir(&root) else {
        return Vec::new();
    };
    let mut found: Vec<(String, String)> = entries
        .filter_map(Result::ok)
        .filter_map(|entry| {
            let name = entry.file_name().to_str()?.to_string();
            let text = std::fs::read_to_string(entry.path().join("solution.patch")).ok()?;
            Some((name, text))
        })
        .collect();
    found.sort();
    found
}
