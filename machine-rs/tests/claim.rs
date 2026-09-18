// A claim must render exactly what the engine accepted when measured: one
// register, several registers, an alternative, and a memory observation.
use hyperray_machine::claim::{negated, Claim, Place};

fn register(name: &str, value: u64) -> Claim {
    Claim::Equals(Place::Register(name.to_string()), value)
}

#[test]
fn one_register_renders_as_the_engine_accepts() {
    let claim = register("R0", 42);
    assert_eq!(claim.to_string(), "0:R0 = 0x000000000000002a");
}

#[test]
fn a_memory_observation_uses_the_star_form() {
    let claim = Claim::Equals(Place::Memory("slot".to_string()), 42);
    assert_eq!(claim.to_string(), "*slot = 0x000000000000002a");
}

#[test]
fn several_places_join_with_and() {
    let claim = Claim::All(vec![register("R0", 42), register("_PC", 0x100010000)]);
    assert_eq!(
        claim.to_string(),
        "(0:R0 = 0x000000000000002a & 0:_PC = 0x0000000100010000)"
    );
}

#[test]
fn alternatives_join_with_or() {
    let claim = Claim::Any(vec![register("R0", 42), register("R0", 99)]);
    assert_eq!(
        claim.to_string(),
        "(0:R0 = 0x000000000000002a | 0:R0 = 0x0000000000000063)"
    );
}

#[test]
fn the_engine_is_asked_to_refute_the_negation() {
    assert_eq!(negated(&register("R0", 0)), "~(0:R0 = 0x0000000000000000)");
}
