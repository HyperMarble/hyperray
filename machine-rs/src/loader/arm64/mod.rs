// AArch64 facts, from the Arm Procedure Call Standard: a result returns in
// X0, and BL puts the return address in X30.
use crate::loader::target::{ReturnAddress, Target, TargetName};

pub const TARGET: Target = Target {
    name: TargetName::Arm64,
    result_register: "R0",
    return_address: ReturnAddress::Register("R30"),
    stack_pointer: "SP_EL0",
    program_counter: "_PC",
};
