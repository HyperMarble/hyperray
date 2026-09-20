// Confirms a counterexample on the processor that shipped the code.
// A proof holds for every input, so only a counterexample can be rechecked.
pub mod abi;
pub mod call;
pub mod page;
pub mod run;
pub mod witness;

pub use call::{Buffer, Call, CallError, Outcome};
pub use run::run;
pub use witness::{compare, named_value, Recheck, RecheckError};
