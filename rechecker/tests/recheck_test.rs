// A counterexample must be reproduced by the processor that shipped the code.
// What an instruction does comes from the model, never from a mask here.
use rechecker::{run, Buffer, Call, Outcome, RecheckError};
use solver::Facts;

/// The names the ARM model gives the argument registers and the stack pointer.
const ARM_ARGUMENTS: [&str; 8] = ["X0", "X1", "X2", "X3", "X4", "X5", "X6", "X7"];
const ARM_STACK_POINTER: &str = "SP_EL0";

/// add x0, x1, x0 ; ret  -- what the compiler emitted for next_offset.
const NEXT_OFFSET: &[u32] = &[0x8b00_0020, 0xd65f_03c0];
/// add x0, x0, x1 ; add x0, x0, x2 ; add x0, x0, x3 ; ret
const SUM_FOUR: &[u32] = &[0x8b01_0000, 0x8b02_0000, 0x8b03_0000, 0xd65f_03c0];
/// ldrb w2, [x0] ; strb w2, [x1] ; ret  -- copies one byte through pointers.
const COPY_BYTE: &[u32] = &[0x3940_0002, 0x3900_0022, 0xd65f_03c0];

fn fact(reads: &[&str], writes: &[&str], load: bool, branch: bool) -> Facts {
    Facts {
        register_reads: reads.iter().map(|s| s.to_string()).collect(),
        register_writes: writes.iter().map(|s| s.to_string()).collect(),
        is_load: load,
        is_store: false,
        is_branch: branch,
    }
}

/// The model reports a return as a branch that writes no register.
fn returning() -> Facts {
    fact(&["R30"], &[], false, true)
}

fn arithmetic() -> Facts {
    fact(&["R0", "R1"], &["R0"], false, false)
}

fn plain(arguments: Vec<u64>) -> Call {
    Call { arguments, buffers: Vec::new() }
}

#[test]
fn the_witness_places_the_registers_the_solver_named() {
    let named = vec![("X1".to_string(), 500), ("X0".to_string(), u64::MAX)];
    let values = rechecker::abi::argument_values(&named, &ARM_ARGUMENTS);
    assert_eq!(values[0], u64::MAX, "the witness order must not matter");
    assert_eq!(values[1], 500);
    assert_eq!(values[2], 0, "a register the witness did not name is zero");
}

#[test]
fn the_processor_reproduces_the_wrap_the_solver_found() {
    let facts = vec![arithmetic(), returning()];
    let outcome = run(NEXT_OFFSET, &plain(vec![u64::MAX, 500]), &facts, ARM_STACK_POINTER).expect("the code must run");
    assert_eq!(outcome.returned, 499, "the chip must produce the wrap the solver named");
}

#[test]
fn a_function_with_four_arguments_runs() {
    let facts = vec![arithmetic(), arithmetic(), arithmetic(), returning()];
    let outcome = run(SUM_FOUR, &plain(vec![1, 2, 3, 4]), &facts, ARM_STACK_POINTER).expect("the code must run");
    assert_eq!(outcome.returned, 10);
}

#[test]
fn a_function_that_writes_through_a_pointer_reports_what_it_wrote() {
    let facts = vec![fact(&["R0"], &["R2"], true, false), arithmetic(), returning()];
    let call = Call {
        arguments: vec![0, 0],
        buffers: vec![
            Buffer { argument: 0, bytes: vec![0x41] },
            Buffer { argument: 1, bytes: vec![0x00] },
        ],
    };
    let Outcome { buffers, .. } = run(COPY_BYTE, &call, &facts, ARM_STACK_POINTER).expect("the code must run");
    assert_eq!(buffers[1], vec![0x41], "the byte written through the pointer must be reported");
}

#[test]
fn a_function_reading_caller_memory_is_refused() {
    let facts = vec![fact(&[ARM_STACK_POINTER], &["R9"], true, false), returning()];
    match run(NEXT_OFFSET, &plain(vec![1, 2]), &facts, ARM_STACK_POINTER) {
        Err(RecheckError::Unavailable(reason)) => {
            assert!(reason.contains("did not place"), "the reason must say what was not placed")
        }
        other => panic!("a function reading caller memory must be refused, got {other:?}"),
    }
}

#[test]
fn code_that_does_not_return_is_refused() {
    match run(NEXT_OFFSET, &plain(vec![1, 1]), &[arithmetic()], ARM_STACK_POINTER) {
        Err(RecheckError::Unavailable(reason)) => assert!(reason.contains("return")),
        other => panic!("code without a return must be refused, got {other:?}"),
    }
}

#[test]
fn a_buffer_naming_an_argument_that_does_not_exist_is_refused() {
    let facts = vec![arithmetic(), returning()];
    let call = Call { arguments: vec![0], buffers: vec![Buffer { argument: 3, bytes: vec![0] }] };
    match run(NEXT_OFFSET, &call, &facts, ARM_STACK_POINTER) {
        Err(RecheckError::Unavailable(reason)) => {
            assert!(reason.contains("BufferWithoutArgument"))
        }
        other => panic!("a buffer without an argument must be refused, got {other:?}"),
    }
}
