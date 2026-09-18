// The x86-64 convention a proof relies on, as the System V AMD64 ABI states
// it. The return address is pushed, not held in a register.
use hyperray_machine::loader::{x86_64, ReturnAddress, TargetName};

#[test]
fn a_result_returns_in_rax_and_the_return_address_is_pushed() {
    assert_eq!(x86_64::TARGET.name, TargetName::X86_64);
    assert_eq!(x86_64::TARGET.result_register, "RAX");
    assert_eq!(x86_64::TARGET.return_address, ReturnAddress::OnStack);
}
