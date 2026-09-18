// The AArch64 convention a proof relies on, as the Arm Procedure Call
// Standard states it.
use hyperray_machine::loader::{arm64, ReturnAddress, TargetName};

#[test]
fn a_result_returns_in_x0_and_the_return_address_sits_in_x30() {
    assert_eq!(arm64::TARGET.name, TargetName::Arm64);
    assert_eq!(arm64::TARGET.result_register, "R0");
    assert_eq!(arm64::TARGET.return_address, ReturnAddress::Register("R30"));
}
