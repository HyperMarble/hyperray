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

// Every Charon output under HYPERRAY_FIXTURE_SRC/<tree>/target/hyperray/,
// one per crate, written by the stage-1 live test.
#[allow(dead_code)]
pub fn ullbc_files(tree: &std::path::Path) -> Vec<PathBuf> {
    let dir = tree.join("target").join("hyperray");
    let Ok(entries) = std::fs::read_dir(&dir) else {
        return Vec::new();
    };
    let mut found: Vec<PathBuf> = entries
        .filter_map(Result::ok)
        .map(|e| e.path())
        .filter(|p| p.extension().is_some_and(|x| x == "ullbc"))
        .collect();
    found.sort();
    found
}

// The source tree for a fixture: HYPERRAY_FIXTURE_SRC/<first word of name>.
#[allow(dead_code)]
pub fn tree_for(sources: &std::path::Path, fixture: &str) -> Option<PathBuf> {
    fixture.split('-').next().map(|n| sources.join(n))
}

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
