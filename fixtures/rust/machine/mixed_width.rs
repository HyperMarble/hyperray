// Byte stores and a word read must retain the same little-endian value.
// The compiler supplies all memory instructions.
#![no_std]

#[repr(align(2))]
struct Word {
    bytes: [u8; 2],
}

#[no_mangle]
pub extern "C" fn _start(value: u64) -> u64 {
    let mut word = Word { bytes: [0; 2] };
    let word = core::hint::black_box(&mut word);
    word.bytes[0] = value as u8;
    core::hint::black_box(&mut *word);
    word.bytes[1] = (value >> 8) as u8;
    u16::from_le_bytes(core::hint::black_box(&*word).bytes) as u64
}

#[test]
fn declared_result() {
    assert_eq!(_start(0x2211), 0x2211);
}
