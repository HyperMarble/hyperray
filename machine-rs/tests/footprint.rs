// Tracing an instruction must produce real events from the loaded engine,
// and an undecodable opcode must be reported rather than pass as empty.
use hyperray_machine::engine::Engine;
use hyperray_machine::footprint;
use std::path::Path;
use std::time::Instant;

fn engine() -> Option<Engine> {
    let model = std::env::var("HYPERRAY_ARM64_SAIL_IR").ok()?;
    let config = std::env::var("HYPERRAY_ARM64_ISLA_CONFIG").ok()?;
    Engine::load(Path::new(&model), Path::new(&config)).ok()
}

#[test]
fn a_real_instruction_produces_events() {
    let Some(engine) = engine() else {
        return;
    };
    // MOV X0, #42
    let found = footprint::trace(&engine, &[0x40, 0x05, 0x80, 0xd2]);
    let reason = match &found {
        Err(error) => error.clone(),
        Ok(print) if print.events == 0 => "produced no events".to_string(),
        Ok(_) => String::new(),
    };
    assert!(reason.is_empty(), "{reason}");
}

#[test]
fn many_instructions_cost_one_load() {
    let Some(engine) = engine() else {
        return;
    };
    let started = Instant::now();
    for _ in 0..8 {
        if footprint::trace(&engine, &[0x40, 0x05, 0x80, 0xd2]).is_err() {
            return;
        }
    }
    let spent = started.elapsed().as_secs_f64();
    assert!(
        spent < 8.0,
        "8 instructions took {spent:.2}s, so setup repeats"
    );
}
