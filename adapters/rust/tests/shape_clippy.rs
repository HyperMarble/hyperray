mod common;

use hyperray_rust::extract::{change, crate_dir, manifest};
use hyperray_rust::shape::{findings_in, run};

// The log was captured verbatim from `cargo clippy --message-format=json`
// on a real crate with the six shape lints on.
#[test]
fn clippy_json_yields_only_shape_lints_with_a_primary_span() {
    let Some(root) = common::dir("HYPERRAY_FIXTURES") else {
        return;
    };
    let Ok(log) = std::fs::read_to_string(root.join("clippy-noodles-util.jsonl")) else {
        return;
    };
    let findings = findings_in(&log);
    assert!(!findings.is_empty());
    for finding in &findings {
        assert!(finding.lint.starts_with("clippy::"), "{}", finding.lint);
        assert!(finding.path.ends_with(".rs"), "{}", finding.path);
        assert!(finding.line_start >= 1 && finding.line_start <= finding.line_end);
        assert!(!finding.message.is_empty());
    }
}

#[test]
fn a_non_json_line_is_skipped() {
    assert!(findings_in("Compiling foo\nwarning: bar\n").is_empty());
}

// The rule: clippy runs on the crate of every patched file, and every
// finding inside a changed function lands within that function's span.
#[test]
fn every_finding_in_a_changed_function_sits_inside_its_span() {
    let Some(sources) = common::dir("HYPERRAY_FIXTURE_SRC") else {
        return;
    };
    let mut ran = 0;
    for (name, text) in common::patches() {
        let Some(tree) = name.split('-').next().map(|n| sources.join(n)) else {
            continue;
        };
        let Ok(built) = manifest(&tree, &change(&text)) else {
            continue;
        };
        let Some(first) = built.functions.first() else {
            continue;
        };
        let Some(dir) = crate_dir(&tree, &first.path) else {
            continue;
        };
        let conf = tree.join("target").join("hyperray");
        assert!(std::fs::create_dir_all(&conf).is_ok());
        let Ok(done) = run(&dir, &conf) else {
            panic!("{name}: clippy did not run");
        };
        assert_eq!(done.exit_code, Some(0), "{name}");
        for finding in &done.findings {
            let in_tree = dir.join(&finding.path);
            let Ok(relative) = in_tree.strip_prefix(&tree) else {
                continue;
            };
            let relative = relative.display().to_string();
            let owner = built.functions.iter().find(|f| {
                f.path == relative && (f.start_line..=f.end_line).contains(&finding.line_start)
            });
            if let Some(f) = owner {
                assert!(
                    finding.line_end <= f.end_line,
                    "{}: {}",
                    f.name,
                    finding.lint
                );
            }
        }
        ran += 1;
    }
    assert!(ran > 0, "no fixture had a source tree");
}
