// Cargo artifacts must identify the generated static library unambiguously.
// A dependency artifact or another target cannot replace the binding archive.
use super::select;
use serde_json::json;
use std::io::Cursor;
use std::path::Path;

#[test]
fn reject_wrong_target_or_filename() {
    for (name, kinds, filenames) in [
        ("other", vec!["staticlib"], vec!["/output/libbinding.a"]),
        ("binding", vec!["rlib"], vec!["/output/libbinding.a"]),
        ("binding", vec!["staticlib"], vec!["libbinding.a"]),
        ("binding", vec!["staticlib"], vec!["/output/binding.so"]),
        ("binding", vec!["staticlib"], vec![]),
        (
            "binding",
            vec!["staticlib"],
            vec!["/output/a.a", "/output/b.a"],
        ),
    ] {
        let artifact = json!({"reason":"compiler-artifact",
            "manifest_path":"/output/Cargo.toml",
            "target":{"name":name,"crate_types":kinds},"filenames":filenames});
        let finished = json!({"reason":"build-finished","success":true});
        let records = format!("{artifact}\n{finished}");
        assert!(select(Cursor::new(records), Path::new("/output/Cargo.toml")).is_err());
    }
}
