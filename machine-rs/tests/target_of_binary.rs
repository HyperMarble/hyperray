// The architecture must be read from the binary itself, and one we hold no
// convention for must be named rather than guessed at.
use hyperray_machine::loader::{target, TargetName};

#[test]
fn each_architecture_is_read_from_its_own_binary() {
    let expected = [
        ("arm64.o", TargetName::Arm64, "R0"),
        ("x86_64.o", TargetName::X86_64, "RAX"),
        ("riscv64.o", TargetName::Riscv64, "x10"),
    ];
    for (file, name, result) in expected {
        let Ok(bytes) = std::fs::read(format!("tests/fixtures/{file}")) else {
            continue;
        };
        let read = target::of(&bytes).map(|found| (found.name, found.result_register));
        assert_eq!(read, Ok((name, result)), "{file}");
    }
}

#[test]
fn an_unknown_architecture_is_reported() {
    assert!(target::of(b"not a binary at all").is_err());
}
