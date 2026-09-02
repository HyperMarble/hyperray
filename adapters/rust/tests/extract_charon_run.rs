use hyperray_rust::extract::{change, join, manifest, module_of, run, seen_in, Scope, Status};
use std::path::{Path, PathBuf};

fn env_dir(name: &str) -> Option<PathBuf> {
    let path = PathBuf::from(std::env::var(name).ok()?);
    path.exists().then_some(path)
}

// The crate is the nearest ancestor of a patched path with a Cargo.toml.
fn crate_dir(root: &Path, patched: &str) -> Option<PathBuf> {
    let mut dir = root.join(patched);
    while dir.pop() {
        if dir.join("Cargo.toml").is_file() {
            return Some(dir);
        }
    }
    None
}

#[test]
fn module_paths_follow_rustc_file_rules() {
    assert_eq!(module_of("src/lib.rs"), "crate");
    assert_eq!(module_of("src/a/mod.rs"), "crate::a");
    assert_eq!(module_of("src/a/b.rs"), "crate::a::b");
}

// The rule: after one Charon run, every function the patch touches is
// either extracted or refused with Charon's own reason. None is missing.
#[test]
fn every_patched_function_is_extracted_or_refused_by_name() {
    let (Some(charon), Some(sources)) =
        (env_dir("HYPERRAY_CHARON"), env_dir("HYPERRAY_FIXTURE_SRC"))
    else {
        eprintln!("skipped: HYPERRAY_CHARON or HYPERRAY_FIXTURE_SRC is unset");
        return;
    };
    let patch =
        PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../fixtures/noodles-296/solution.patch");
    let text = std::fs::read_to_string(patch).unwrap_or_default();
    let root = sources.join("noodles");
    let Ok(built) = manifest(&root, &change(&text)) else {
        return;
    };
    let file = "noodles-util/src/alignment/async/io/indexed_reader/builder.rs";
    let Some(dir) = crate_dir(&root, file) else {
        return;
    };
    let in_crate = file.strip_prefix("noodles-util/").unwrap_or(file);
    let modules = vec![module_of(in_crate)];
    let cargo_args = vec!["--features".to_string(), "alignment,async".to_string()];
    let scope = Scope {
        crate_dir: &dir,
        modules: &modules,
        cargo_args: &cargo_args,
    };
    let output = std::env::temp_dir().join("hyperray-noodles.ullbc");
    let done = run(&charon, &scope, &output);
    assert_eq!(done.exit_code, Some(0));
    let ullbc = std::fs::read_to_string(&output).unwrap_or_default();
    let Ok(seen) = seen_in(&ullbc) else {
        panic!("charon output did not parse");
    };
    let in_file: Vec<_> = built
        .functions
        .iter()
        .filter(|f| f.path == file)
        .cloned()
        .collect();
    let joined = join(&in_file, &seen);
    assert!(!joined.is_empty());
    for function in &joined {
        match &function.status {
            Status::Extracted => {}
            Status::Refused(reason) => assert!(!reason.is_empty(), "{}", function.name),
            Status::Missing => panic!("{} missing from charon output", function.name),
        }
    }
    let refused = joined
        .iter()
        .filter(|f| matches!(f.status, Status::Refused(_)));
    let async_fns = in_file.iter().filter(|f| f.text.contains("async fn"));
    assert_eq!(refused.count(), async_fns.count());
}
