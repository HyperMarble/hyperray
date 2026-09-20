// Every function shape the rechecker claims to handle must run on the chip.
// A shape it cannot run must be refused, never answered with a wrong value.
use rechecker::{run, Call, RecheckError};
use solver::Facts;

const ARM_STACK_POINTER: &str = "SP_EL0";

fn fact(reads: &[&str], writes: &[&str], load: bool, branch: bool) -> Facts {
    Facts {
        register_reads: reads.iter().map(|s| s.to_string()).collect(),
        register_writes: writes.iter().map(|s| s.to_string()).collect(),
        is_load: load,
        is_store: false,
        is_branch: branch,
    }
}

fn arithmetic() -> Facts {
    fact(&["R0", "R1"], &["R0"], false, false)
}

fn returning() -> Facts {
    fact(&["R30"], &[], false, true)
}

/// Runs one compiled function with the values given.
fn call(instructions: &[u32], arguments: Vec<u64>) -> u64 {
    let mut facts: Vec<Facts> = (1..instructions.len()).map(|_| arithmetic()).collect();
    facts.push(returning());
    let call = Call { arguments, buffers: Vec::new() };
    run(instructions, &call, &facts, ARM_STACK_POINTER).expect("the code must run").returned
}

#[test]
fn addition_runs() {
    assert_eq!(call(&[0x8b00_0020, 0xd65f_03c0], vec![7, 5]), 12);
}

#[test]
fn subtraction_runs() {
    assert_eq!(call(&[0xcb01_0000, 0xd65f_03c0], vec![9, 4]), 5);
}

#[test]
fn multiplication_of_three_runs() {
    assert_eq!(call(&[0x9b00_7c28, 0x9b02_7d00, 0xd65f_03c0], vec![2, 3, 4]), 24);
}

#[test]
fn a_shift_runs() {
    assert_eq!(call(&[0xd343_fc00, 0xd65f_03c0], vec![64]), 8);
}

#[test]
fn a_comparison_runs() {
    assert_eq!(call(&[0xeb01_001f, 0x9a81_3000, 0xd65f_03c0], vec![9, 4]), 4);
}

#[test]
fn six_arguments_run() {
    let code = [0x8b00_0028, 0x8b03_0049, 0x8b09_0108, 0x8b05_0089, 0x8b09_0100, 0xd65f_03c0];
    assert_eq!(call(&code, vec![1, 2, 3, 4, 5, 6]), 21);
}

#[test]
fn more_arguments_than_the_host_places_are_refused() {
    let call = Call { arguments: vec![0; 9], buffers: Vec::new() };
    let facts = vec![arithmetic(), returning()];
    match run(&[0x8b00_0020, 0xd65f_03c0], &call, &facts, ARM_STACK_POINTER) {
        Err(RecheckError::Unavailable(reason)) => assert!(reason.contains("registers")),
        other => panic!("more arguments than the host places must be refused, got {other:?}"),
    }
}
