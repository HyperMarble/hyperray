// What a proof must know about an architecture that the binary format does
// not say: where a function's result appears, and where it returns to.

/// The architectures a proof can be run for.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TargetName {
    Arm64,
    X86_64,
    Riscv64,
}

/// Where a function's return address is kept when it is called.
///
/// x86-64 pushes it; ARM and RISC-V put it in a register. A proof sets up the
/// wrong state if that difference is assumed rather than named.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ReturnAddress {
    Register(&'static str),
    OnStack,
}

/// The register names one architecture uses, as its published calling
/// convention defines them.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Target {
    pub name: TargetName,
    pub result_register: &'static str,
    pub return_address: ReturnAddress,
    pub stack_pointer: &'static str,
    pub program_counter: &'static str,
}
