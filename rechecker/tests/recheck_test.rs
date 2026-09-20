// A counterexample must be reproduced by the processor that shipped the code.
// Inputs the rechecker cannot pass must be refused, never guessed at.
use rechecker::{run, Buffer, Call, Outcome, RecheckError};

/// add x0, x1, x0 ; ret  -- what the compiler emitted for next_offset.
const NEXT_OFFSET: &[u32] = &[0x8b00_0020, 0xd65f_03c0];
/// add x0, x0, x1 ; add x0, x0, x2 ; add x0, x0, x3 ; ret
const SUM_FOUR: &[u32] = &[0x8b01_0000, 0x8b02_0000, 0x8b03_0000, 0xd65f_03c0];
/// ldrb w2, [x0] ; strb w2, [x1] ; ret  -- copies one byte through pointers.
const COPY_BYTE: &[u32] = &[0x3940_0002, 0x3900_0022, 0xd65f_03c0];

fn plain(arguments: Vec<u64>) -> Call {
    Call { arguments, buffers: Vec::new() }
}

#[test]
fn the_processor_reproduces_the_wrap_the_solver_found() {
    let outcome = run(NEXT_OFFSET, &plain(vec![u64::MAX, 500])).expect("the code must run");
    assert_eq!(outcome.returned, 499, "the chip must produce the wrap the solver named");
}

#[test]
fn a_function_with_four_arguments_runs() {
    let outcome = run(SUM_FOUR, &plain(vec![1, 2, 3, 4])).expect("the code must run");
    assert_eq!(outcome.returned, 10);
}

#[test]
fn a_function_that_writes_through_a_pointer_reports_what_it_wrote() {
    let call = Call {
        arguments: vec![0, 0],
        buffers: vec![
            Buffer { argument: 0, bytes: vec![0x41] },
            Buffer { argument: 1, bytes: vec![0x00] },
        ],
    };
    let Outcome { buffers, .. } = run(COPY_BYTE, &call).expect("the code must run");
    assert_eq!(buffers[1], vec![0x41], "the byte written through the pointer must be reported");
}

/// ldp x9, x8, [sp] ; add x0, x0, x9 ; add x0, x0, x8 ; ret
/// The ninth and tenth arguments are read from the stack, as the compiler
/// emits for a function with ten arguments.
const READS_STACK: &[u32] = &[0xa940_23e9, 0x8b09_0000, 0x8b08_0000, 0xd65f_03c0];

#[test]
fn a_function_reading_stack_arguments_is_refused() {
    match run(READS_STACK, &plain(vec![1, 0, 0, 0, 0, 0, 0, 0, 20, 300])) {
        Err(RecheckError::Unavailable(reason)) => {
            assert!(reason.contains("did not place"), "the reason must say what was not placed")
        }
        other => panic!("a function reading the stack must be refused, got {other:?}"),
    }
}

#[test]
fn a_buffer_naming_an_argument_that_does_not_exist_is_refused() {
    let call = Call { arguments: vec![0], buffers: vec![Buffer { argument: 3, bytes: vec![0] }] };
    match run(NEXT_OFFSET, &call) {
        Err(RecheckError::Unavailable(reason)) => {
            assert!(reason.contains("BufferWithoutArgument"))
        }
        other => panic!("a buffer without an argument must be refused, got {other:?}"),
    }
}

#[test]
fn code_that_does_not_return_is_refused() {
    match run(&[0x8b00_0020], &plain(vec![1, 1])) {
        Err(RecheckError::Unavailable(reason)) => assert!(reason.contains("return")),
        other => panic!("code without a return must be refused, got {other:?}"),
    }
}
