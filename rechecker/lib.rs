// Purpose: confirms a counterexample on the processor that shipped the code.
// Never:   rechecks a proof, which holds for every input and names no case.
// In:      nothing, this file only names the parts
// Out:     the call types, the runner, and the comparison
// Fails:   not applicable
pub mod abi;
#[path = "arm64/call.rs"]
pub mod arm64_call;
pub mod call;
pub mod placed;
pub mod run_placed;
pub mod witness;

pub use call::{Buffer, Call, CallError, Outcome};
pub use placed::Placed;
pub use run_placed::run_placed;
pub use witness::{compare, named_value, Recheck, RecheckError};
