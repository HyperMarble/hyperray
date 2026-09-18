// A trait-object call preserves a compiler-generated indirect target.
// The proof must retain the vtable and object memory.
#![no_std]

trait Transform {
    fn apply(&self, value: u64) -> u64;
}

struct Increase(u64);

impl Transform for Increase {
    #[inline(never)]
    fn apply(&self, value: u64) -> u64 {
        value.wrapping_add(self.0)
    }
}

#[inline(never)]
fn dispatch(transform: &dyn Transform, value: u64) -> u64 {
    core::hint::black_box(transform).apply(value)
}

#[no_mangle]
pub extern "C" fn _start(value: u64) -> u64 {
    dispatch(&Increase(7), value)
}

#[test]
fn declared_result() {
    assert_eq!(_start(5), 12);
}
