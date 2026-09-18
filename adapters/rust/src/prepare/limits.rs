// Bounds describe the caller's input interval and explicit search resources.
// Resource limits never establish search completeness.
use serde::{Deserialize, Serialize};

const MAXIMUM_RUNTIME_BYTES: u64 = 100_000_000;
const BYTES_PER_MEBIBYTE: u64 = 1024 * 1024;
const NANOSECONDS_PER_MILLISECOND: u64 = 1_000_000;

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct Limits {
    pub minimum: u64,
    pub maximum: u64,
    pub search_depth: u32,
    pub hash_bits: u8,
    pub state_memory_mib: u32,
    pub timeout_ms: u64,
    pub output_limit_bytes: u64,
    pub memory_budget_bytes: u64,
}

impl Limits {
    pub fn validate(&self) -> Result<(), String> {
        input_bits(self.minimum, self.maximum)?;
        if self.search_depth == 0 || self.search_depth > i32::MAX as u32 {
            return Err("search depth must fit a positive signed 32-bit value".into());
        }
        if self.hash_bits == 0 || u32::from(self.hash_bits) >= usize::BITS {
            return Err("hash bits must fit a positive native shift".into());
        }
        if self.memory_budget_bytes == 0 || self.memory_budget_bytes > MAXIMUM_RUNTIME_BYTES {
            return Err("runtime memory budget must be within 1..100000000 bytes".into());
        }
        let state_bytes = u64::from(self.state_memory_mib) * BYTES_PER_MEBIBYTE;
        if state_bytes == 0 || state_bytes > self.memory_budget_bytes {
            return Err("state memory must be positive and fit the runtime budget".into());
        }
        if self.output_limit_bytes == 0 || self.output_limit_bytes > self.memory_budget_bytes {
            return Err("output limit must be positive and fit the runtime budget".into());
        }
        self.timeout_nanoseconds()?;
        Ok(())
    }

    pub fn timeout_nanoseconds(&self) -> Result<i64, String> {
        let value = self.timeout_ms.checked_mul(NANOSECONDS_PER_MILLISECOND);
        match value.and_then(|value| i64::try_from(value).ok()) {
            Some(value) if value > 0 => Ok(value),
            Some(_) | None => Err("timeout must fit a positive signed nanosecond duration".into()),
        }
    }
}

pub fn input_bits(minimum: u64, maximum: u64) -> Result<u32, String> {
    let span = maximum
        .checked_sub(minimum)
        .ok_or("input interval is reversed")?;
    Ok(u64::BITS - span.leading_zeros())
}
