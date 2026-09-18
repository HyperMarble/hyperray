// The RISC-V convention a proof relies on, as the RISC-V calling convention
// states it.
use hyperray_machine::loader::{riscv64, ReturnAddress, TargetName};

#[test]
fn a_result_returns_in_a0_and_the_return_address_sits_in_ra() {
    assert_eq!(riscv64::TARGET.name, TargetName::Riscv64);
    assert_eq!(riscv64::TARGET.result_register, "x10");
    assert_eq!(
        riscv64::TARGET.return_address,
        ReturnAddress::Register("x1")
    );
}
