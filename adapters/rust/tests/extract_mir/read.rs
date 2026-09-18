// MIR reading joins compiler dumps to patch functions without guessed rows.

use crate::extract_mir_compile::compile_crate;
use hyperray_rust::extract::{change, crate_dir, join, manifest, seen_in, Global, Joined, Read};
use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

pub struct Checked {
    pub joined: Vec<Joined>,
    pub globals: Vec<Global>,
}

pub fn read_all(driver: &Path, root: &Path, patch: &str) -> Result<Checked, String> {
    let changes = change(patch);
    let Ok(built) = manifest(root, &changes) else {
        return Err(format!("manifest failed for {}", root.display()));
    };
    let crates: BTreeSet<PathBuf> = built
        .functions
        .iter()
        .filter_map(|function| crate_dir(root, &function.path))
        .collect();
    let work = root.join("target").join("hyperray-mir");
    let read = read_crates(driver, &crates, &work)?;
    Ok(Checked {
        joined: join(&built.functions, &read.functions),
        globals: read.globals,
    })
}

fn read_crates(driver: &Path, crates: &BTreeSet<PathBuf>, work: &Path) -> Result<Read, String> {
    if let Err(error) = std::fs::create_dir_all(work) {
        return Err(format!("{}: {error}", work.display()));
    }
    let mut dumps = Vec::new();
    for dir in crates {
        let done = compile_crate(driver, dir, work)?;
        if done.exit_code != Some(0) {
            return Err(format!("{}\n{}", dir.display(), done.log));
        }
        if done.dumps == 0 {
            return Err(format!("{}: the compiler wrote nothing", dir.display()));
        }
        dumps.extend(done.dump_paths);
    }
    collect(&dumps)
}

fn collect(dumps: &[PathBuf]) -> Result<Read, String> {
    let mut read = Read {
        functions: Vec::new(),
        globals: Vec::new(),
    };
    for dump in dumps {
        let part = one_dump(dump)?;
        read.functions.extend(part.functions);
        read.globals.extend(part.globals);
    }
    Ok(read)
}

fn one_dump(dump: &Path) -> Result<Read, String> {
    let Ok(file) = std::fs::File::open(dump) else {
        return Err(format!("cannot open {}", dump.display()));
    };
    match seen_in(file) {
        Ok(read) => Ok(read),
        Err(error) => Err(format!("{}: {error}", dump.display())),
    }
}
