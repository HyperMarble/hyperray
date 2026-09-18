// Stage 4 runner tests cover captured logs and configured live Kani crates.
// External fixture paths remain outside the adapter.

mod common;
#[path = "prove_kani/live.rs"]
mod live;
#[path = "prove_kani/logs.rs"]
mod logs;
