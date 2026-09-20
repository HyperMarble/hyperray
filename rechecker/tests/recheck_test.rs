// A counterexample must be reproduced by the processor that shipped the code.
// Instructions the rechecker cannot run must be refused, never guessed at.
use rechecker::run::run_arm64;
use rechecker::witness::{compare, named_value, Recheck, RecheckError};
use solver::Witness;

/// add x0, x1, x0 ; ret  -- the instructions the compiler emitted for
/// next_offset, which the solver disproved at u64::MAX.
const NEXT_OFFSET: &[u32] = &[0x8b00_0020, 0xd65f_03c0];

#[test]
fn the_processor_reproduces_the_wrap_the_solver_found() {
    let produced = run_arm64(NEXT_OFFSET, u64::MAX, 500).expect("the code must run");
    assert_eq!(
        compare(produced, 499),
        Recheck::Confirmed { produced: 499 },
        "the chip must produce what the solver named"
    );
}

#[test]
fn a_value_the_chip_does_not_produce_is_contradicted() {
    let produced = run_arm64(NEXT_OFFSET, 8, 500).expect("the code must run");
    match compare(produced, 499) {
        Recheck::Contradicted { produced, expected } => {
            assert_eq!((produced, expected), (508, 499))
        }
        other => panic!("a different value must be contradicted, got {other:?}"),
    }
}

#[test]
fn code_that_does_not_return_is_refused() {
    match run_arm64(&[0x8b00_0020], 1, 1) {
        Err(RecheckError::Unavailable(reason)) => assert!(reason.contains("return")),
        other => panic!("code without a return must be refused, got {other:?}"),
    }
}

#[test]
fn a_witness_without_the_register_is_reported() {
    let witness = Witness { registers: vec![("X0".to_string(), 5)] };
    assert_eq!(named_value(&witness, "X0"), Ok(5));
    match named_value(&witness, "X1") {
        Err(RecheckError::MissingRegister(name)) => assert_eq!(name, "X1"),
        other => panic!("a missing register must be named, got {other:?}"),
    }
}
