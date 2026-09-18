// Loading a binary is one job for every architecture: the object format
// carries the bytes. What differs per architecture is named here.
pub mod arm64;
pub mod riscv64;
pub mod target;
pub mod x86_64;

pub use target::{ReturnAddress, Target, TargetName};
