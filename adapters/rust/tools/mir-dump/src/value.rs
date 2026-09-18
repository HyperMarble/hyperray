// A static value read from the compiler memory image. Non-scalar values have
// no value in this boundary.

use rustc_public::mir::mono::StaticDef;
use rustc_public::CrateItem;

pub fn of(item: &CrateItem) -> Option<String> {
    let definition = StaticDef::try_from(*item).ok()?;
    let memory = definition.eval_initializer().ok()?;
    let bytes = memory.raw_bytes().ok()?;
    memory
        .read_partial_uint(0..bytes.len())
        .ok()
        .map(|value| value.to_string())
}
