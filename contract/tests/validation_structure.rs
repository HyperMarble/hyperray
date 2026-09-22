// Purpose: Test the fixed section-order invariant for every adjacent boundary.
// Never: Treat one malformed ordering as evidence for all order failures.
// In: Every adjacent swap and every absent required section group.
// Out: Rejection for each structurally invalid parsed contract.
// Fails: A misplaced, unknown, duplicate, or absent group reaches later checks.

mod support;

use contract::{parse, validate, ValidationFailure};
use proptest::prelude::*;
use support::facts;

fn sections() -> Vec<&'static str> {
    vec![
        "(fn advance)",
        "(in arg1 all)",
        "(out ret)",
        "(mem none)",
        "(os none)",
        "(req true)",
    ]
}

fn rejected(source: &str) -> bool {
    let parsed = parse(source);
    let result = parsed
        .ok()
        .and_then(|value| validate(value, &facts()).err());
    matches!(result, Some(ValidationFailure::Rejected(_)))
}

proptest! {
    #[test]
    fn rejects_every_adjacent_section_swap(boundary in 0usize..5) {
        let mut values = sections();
        values.swap(boundary, boundary + 1);
        let source = format!("(hray {})", values.join(" "));
        prop_assert!(rejected(&source));
    }
}

#[test]
fn rejects_each_absent_required_group() {
    let values = sections();
    let rejected_all = (0..values.len()).all(|missing| {
        let kept = values
            .iter()
            .enumerate()
            .filter(|(index, _)| *index != missing)
            .map(|(_, value)| *value)
            .collect::<Vec<_>>();
        rejected(&format!("(hray {})", kept.join(" ")))
    });
    assert!(rejected_all);
}
