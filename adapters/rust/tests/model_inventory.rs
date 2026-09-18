// Stage 3 model tests cover the CBMC boundary and both Kani harness routes.
// The live test receives its crate path instead of naming a fixture here.

#[path = "model_inventory/decode.rs"]
mod decode;
#[path = "model_inventory/live.rs"]
mod live;
#[path = "model_inventory/support.rs"]
mod support;
