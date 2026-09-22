// Purpose: Test validator invariants across arbitrary compiler argument counts.
// Never: Use one function arity as evidence for all compiler metadata shapes.
// In: Generated complete argument inventories and one generated mutation.
// Out: Acceptance for exact inventories and rejection for missing or duplicate inputs.
// Fails: Validation depends on a fixed argument count or selected argument number.

mod support;

use contract::{FunctionFact, ValidationFacts};
use proptest::prelude::*;
use support::{facts, validation_result, value, FailureKind};

fn facts_with_arguments(count: usize) -> ValidationFacts {
    let mut facts = facts();
    facts.functions = vec![FunctionFact {
        name: "generated".into(),
        arguments: (0..count).map(|_| value()).collect(),
        result: None,
    }];
    facts
}

fn source(arguments: &[usize]) -> String {
    let inputs = if arguments.is_empty() {
        "(in none)".into()
    } else {
        arguments
            .iter()
            .map(|number| format!("(in arg{number} all)"))
            .collect::<Vec<_>>()
            .join(" ")
    };
    format!("(hray (fn generated) {inputs} (out none) (mem none) (os none) (req true))")
}

fn failure_kind(source: &str, facts: &ValidationFacts) -> Option<FailureKind> {
    validation_result(source, facts).err()
}

proptest! {
    #[test]
    fn accepts_every_generated_argument_inventory(count in 0usize..32) {
        let arguments = (1..=count).collect::<Vec<_>>();
        let result = failure_kind(&source(&arguments), &facts_with_arguments(count));
        prop_assert_eq!(result, None);
    }

    #[test]
    fn rejects_each_generated_missing_argument(
        (count, missing) in (1usize..32).prop_flat_map(|count| (Just(count), 1..=count))
    ) {
        let arguments = (1..=count).filter(|number| *number != missing).collect::<Vec<_>>();
        let result = failure_kind(&source(&arguments), &facts_with_arguments(count));
        prop_assert_eq!(result, Some(FailureKind::Rejected));
    }

    #[test]
    fn rejects_each_generated_duplicate_argument(
        (count, duplicate) in (1usize..32).prop_flat_map(|count| (Just(count), 1..=count))
    ) {
        let mut arguments = (1..=count).collect::<Vec<_>>();
        arguments.push(duplicate);
        let result = failure_kind(&source(&arguments), &facts_with_arguments(count));
        prop_assert_eq!(result, Some(FailureKind::Rejected));
    }
}
