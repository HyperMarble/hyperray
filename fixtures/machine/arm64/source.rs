// This source supplies a tiny native ARM64 Mach-O analysis fixture.
// It must not require runtime services or imply ordinary process support.
#![no_std]

#[no_mangle]
pub extern "C" fn arm64_fixture(value: u64) -> u64 {
	value.wrapping_add(1)
}
