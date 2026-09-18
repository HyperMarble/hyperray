// x86-64 facts, from the System V AMD64 ABI: an integer result returns in
// RAX, and CALL pushes the return address onto the stack.
use crate::loader::target::{ReturnAddress, Target, TargetName};

pub const TARGET: Target = Target {
    name: TargetName::X86_64,
    result_register: "RAX",
    return_address: ReturnAddress::OnStack,
    stack_pointer: "RSP",
    program_counter: "RIP",
};
