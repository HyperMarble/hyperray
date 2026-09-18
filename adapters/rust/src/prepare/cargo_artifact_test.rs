// Cargo's documented records define selection, ambiguity, and failure behavior.
// These synthetic protocol records do not replace the real Cargo build matrix.
use super::select;
use serde_json::json;
use std::io::Cursor;
use std::path::Path;

fn artifact(manifest: &str) -> String {
    json!({"reason":"compiler-artifact", "manifest_path":manifest,
        "target":{"name":"binding", "crate_types":["staticlib"]},
        "filenames":["/output/libbinding.a"]})
    .to_string()
}

#[test]
fn select_generated_archive() -> Result<(), String> {
    let messages = format!(
        "{}\n{}\n{}\n",
        artifact("/dependency/Cargo.toml"),
        artifact("/output/Cargo.toml"),
        json!({"reason":"build-finished", "success":true})
    );
    let selected = select(Cursor::new(messages), Path::new("/output/Cargo.toml"))?;
    assert_eq!(selected, Path::new("/output/libbinding.a"));
    Ok(())
}

#[test]
fn reject_missing_or_ambiguous_results() {
    let generated = artifact("/output/Cargo.toml");
    let finished = json!({"reason":"build-finished", "success":true}).to_string();
    let failed = json!({"reason":"build-finished", "success":false}).to_string();
    for messages in [
        String::new(),
        "not JSON".into(),
        generated.clone(),
        finished.clone(),
        format!("{generated}\n{failed}"),
        format!("{generated}\n{generated}\n{finished}"),
        format!("{generated}\n{finished}\n{finished}"),
    ] {
        assert!(select(Cursor::new(messages), Path::new("/output/Cargo.toml")).is_err());
    }
}

#[test]
fn reject_malformed_records() {
    let finished = json!({"reason":"build-finished", "success":true});
    for record in [
        json!({}),
        json!({"reason":"future-format"}),
        json!({"reason":"build-finished"}),
        json!({"reason":"compiler-artifact"}),
    ] {
        let messages = format!("{record}\n{finished}");
        assert!(select(Cursor::new(messages), Path::new("/output/Cargo.toml")).is_err());
    }
}
