// Compile outcomes must match their declared feature inventory.
// Prior dump paths and an empty successful inventory must never pass.
use hyperray_rust::{extract::Run, mir};
use std::collections::{BTreeMap, BTreeSet};
use std::path::{Path, PathBuf};

pub fn inventory(
    result: &Run,
    expected: Result<&str, &str>,
) -> Result<(), Box<dyn std::error::Error>> {
    let expected = match expected {
        Ok(name) => name,
        Err(diagnostic) => {
            assert!(result.exit_code.is_some(), "{}", result.log);
            assert_ne!(result.exit_code, Some(0), "{}", result.log);
            assert!(result.log.contains(diagnostic), "{}", result.log);
            assert_eq!(result.dumps, 0);
            return Ok(());
        }
    };
    assert_eq!(result.exit_code, Some(0), "{}", result.log);
    assert!(result.dumps > 0, "{}", result.log);
    for path in &result.dump_paths {
        let dump = mir::read(std::fs::File::open(path)?)?;
        let names: BTreeSet<String> = dump.items.into_iter().map(|item| item.name).collect();
        inventory_names(&names, expected);
    }
    Ok(())
}

fn inventory_names(names: &BTreeSet<String>, expected: &str) {
    for name in ["left_only", "right_only", "neither"] {
        let full = format!("compiler_feature_selection::{name}");
        assert_eq!(names.contains(&full), name == expected, "{names:?}");
    }
}

pub fn snapshot(directory: &Path) -> Result<BTreeMap<PathBuf, Vec<u8>>, std::io::Error> {
    let mut files = BTreeMap::new();
    for entry in std::fs::read_dir(directory)? {
        let path = entry?.path();
        if path.is_dir() {
            files.extend(snapshot(&path)?);
        } else {
            files.insert(path.clone(), std::fs::read(&path)?);
        }
    }
    Ok(files)
}
