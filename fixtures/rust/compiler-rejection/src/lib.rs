// This crate requires a compiler error to exercise error propagation.
// A failed compiler invocation must not appear successful.

compile_error!("compiler rejection fixture");
