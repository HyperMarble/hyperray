// RISC-V facts, from the RISC-V calling convention: a result returns in a0,
// and a call puts the return address in ra.
use crate::loader::target::{ReturnAddress, Target, TargetName};

pub const TARGET: Target = Target {
    name: TargetName::Riscv64,
    result_register: "x10",
    return_address: ReturnAddress::Register("x1"),
    stack_pointer: "x2",
    program_counter: "PC",
};
