// The same encoding must be traced once. Our own binaries repeat 82% of
// their instructions, so tracing each occurrence is wasted work.
use hyperray_machine::{engine::Engine, traced::Traced};
use std::path::Path;
use std::time::Instant;

fn engine() -> Option<Engine> {
    let model = std::env::var("HYPERRAY_ARM64_SAIL_IR").ok()?;
    let config = std::env::var("HYPERRAY_ARM64_ISLA_CONFIG").ok()?;
    Engine::load(Path::new(&model), Path::new(&config)).ok()
}

#[test]
fn a_repeated_encoding_is_traced_once() {
    let Some(engine) = engine() else {
        return;
    };
    let mut store = Traced::new();
    let ret = 0xd65f03c0u32.to_le_bytes();
    for _ in 0..8 {
        if store.footprint(&engine, ret).is_err() {
            return;
        }
    }
    let (hits, misses) = store.counts();
    assert_eq!((hits, misses), (7, 1), "traced more than once");
}

#[test]
fn a_repeated_encoding_costs_nothing_after_the_first() {
    let Some(engine) = engine() else {
        return;
    };
    let mut store = Traced::new();
    let ret = 0xd65f03c0u32.to_le_bytes();
    if store.footprint(&engine, ret).is_err() {
        return;
    }
    let again = Instant::now();
    for _ in 0..100 {
        let _ = store.footprint(&engine, ret);
    }
    let spent = again.elapsed().as_secs_f64();
    assert!(spent < 0.05, "100 repeats cost {spent:.3}s, so it retraced");
}

#[test]
fn different_encodings_are_traced_separately() {
    let Some(engine) = engine() else {
        return;
    };
    let mut store = Traced::new();
    for word in [0xd65f03c0u32, 0xd2800540, 0xd65f03c0] {
        if store.footprint(&engine, word.to_le_bytes()).is_err() {
            return;
        }
    }
    assert_eq!(store.counts(), (1, 2));
}
