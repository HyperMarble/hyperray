// What an instruction does must come from the model, not from a mask here.
use solver::{reads_caller_memory, returns, Facts};

fn facts(reads: &[&str], writes: &[&str], load: bool, branch: bool) -> Facts {
    Facts {
        register_reads: reads.iter().map(|s| s.to_string()).collect(),
        register_writes: writes.iter().map(|s| s.to_string()).collect(),
        is_load: load,
        is_store: false,
        is_branch: branch,
    }
}

#[test]
fn a_branch_that_writes_no_register_returns() {
    assert!(returns(&facts(&["R30"], &[], false, true)), "ret reads the link register only");
}

#[test]
fn a_branch_that_writes_a_register_does_not_return() {
    assert!(!returns(&facts(&["R30"], &["R30"], false, true)), "a call writes the link register");
}

#[test]
fn an_arithmetic_instruction_does_not_return() {
    assert!(!returns(&facts(&["R0", "R1"], &["R0"], false, false)));
}

#[test]
fn a_load_through_the_stack_pointer_reads_caller_memory() {
    assert!(reads_caller_memory(&facts(&["SP_EL0"], &["R9"], true, false), "SP_EL0"));
}

#[test]
fn a_load_through_another_register_does_not() {
    assert!(!reads_caller_memory(&facts(&["R0"], &["R1"], true, false), "SP_EL0"));
}
