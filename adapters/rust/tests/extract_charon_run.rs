use hyperray_rust::extract::{change, item_path, manifest, run, Scope};
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
fn item_paths_follow_the_file_layout() {
    assert_eq!(item_path("src/lib.rs", None, "f"), "crate::f");
    assert_eq!(item_path("src/a/mod.rs", None, "f"), "crate::a::f");
    assert_eq!(item_path("src/a/b.rs", None, "f"), "crate::a::b::f");
    assert_eq!(item_path("src/a/b.rs", Some("T"), "f"), "crate::a::b::T::f");
}

// Measured on 2026-09-01: Charon refuses exactly the five `async fn` in
// the noodles-util builder and extracts the rest.
#[test]
fn charon_run_on_noodles_refuses_only_the_async_functions() {
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
    let builder = "noodles-util/src/alignment/async/io/indexed_reader/builder.rs";
    let Some(dir) = crate_dir(&root, builder) else {
        return;
    };
    let in_crate = builder.strip_prefix("noodles-util/").unwrap_or(builder);
    let items: Vec<String> = built
        .functions
        .iter()
        .filter(|f| f.path == builder)
        .map(|f| item_path(in_crate, f.owner.as_deref(), &f.name))
        .collect();
    let cargo_args = vec!["--features".to_string(), "alignment,async".to_string()];
    let scope = Scope {
        crate_dir: &dir,
        items: &items,
        cargo_args: &cargo_args,
    };
    let output = std::env::temp_dir().join("hyperray-noodles.ullbc");
    let done = run(&charon, &scope, &output);
    assert_eq!(done.exit_code, Some(0));
    let coroutine_lines: Vec<u32> = done
        .refusals
        .iter()
        .filter(|r| r.reason.starts_with("Coroutine"))
        .filter(|r| r.path.ends_with("builder.rs"))
        .map(|r| r.line)
        .collect();
    let mut refused_functions: Vec<&str> = built
        .functions
        .iter()
        .filter(|f| f.path == builder)
        .filter(|f| {
            coroutine_lines
                .iter()
                .any(|l| (f.start_line..=f.end_line).contains(l))
        })
        .map(|f| f.name.as_str())
        .collect();
    refused_functions.sort_unstable();
    let mut async_functions: Vec<&str> = built
        .functions
        .iter()
        .filter(|f| f.path == builder && f.text.contains("async fn"))
        .map(|f| f.name.as_str())
        .collect();
    async_functions.sort_unstable();
    assert_eq!(refused_functions, async_functions);
    assert_eq!(async_functions.len(), 5);
}
