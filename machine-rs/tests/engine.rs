// Loading the engine once must cost about as much as one load, not three
// parses and two initialisations the way a spawned process does.
use hyperray_machine::engine::Engine;
use std::path::Path;
use std::time::Instant;

fn paths() -> Option<(String, String)> {
    let model = std::env::var("HYPERRAY_ARM64_SAIL_IR").ok()?;
    let config = std::env::var("HYPERRAY_ARM64_ISLA_CONFIG").ok()?;
    Some((model, config))
}

#[test]
fn the_engine_loads_from_the_real_model() {
    let Some((model, config)) = paths() else {
        return;
    };
    let loaded = Engine::load(Path::new(&model), Path::new(&config));
    let reason = match &loaded {
        Err(error) => error.clone(),
        Ok(_) => String::new(),
    };
    assert!(reason.is_empty(), "{reason}");
}

#[test]
fn a_second_stage_costs_nothing_to_start() {
    let Some((model, config)) = paths() else {
        return;
    };
    let Ok(engine) = Engine::load(Path::new(&model), Path::new(&config)) else {
        return;
    };
    let reuse = Instant::now();
    for _ in 0..1000 {
        let _ = engine.ready();
        let _ = engine.config();
    }
    let spent = reuse.elapsed().as_secs_f64();
    assert!(spent < 0.10, "1000 stages cost {spent:.2}s, so it reloads");
}

#[test]
fn an_absent_model_is_a_reported_failure() {
    let found = Engine::load(
        Path::new("/nonexistent/m.ir"),
        Path::new("/nonexistent/c.toml"),
    );
    assert!(found.is_err(), "absent model returned a usable engine");
}
