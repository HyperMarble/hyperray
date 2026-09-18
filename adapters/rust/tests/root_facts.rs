// Root-facts fixtures exercise the additive producer schema.
// They never assert valid machine roots or change legacy inventory behavior.

use hyperray_rust::mir::RawInventory;

#[test]
fn root_facts_fixture_preserves_compiler_abi_evidence() -> Result<(), Box<dyn std::error::Error>> {
    let content = include_str!("../../../fixtures/rust/root-facts/root_facts.json");
    let inventory: RawInventory = serde_json::from_str(content)?;
    let instance = inventory
        .instances
        .first()
        .ok_or("root-facts fixture has no instance")?;
    let facts = instance
        .root_facts
        .as_ref()
        .ok_or("root-facts fixture has no facts")?;
    assert_eq!(facts.version, 1);
    assert_eq!(facts.target, "riscv64gc-unknown-linux-gnu");
    assert_eq!(facts.endian, "little");
    assert_eq!(facts.pointer_bits, 64);
    assert!(facts.requires_caller_location);
    assert_eq!(facts.args.len(), 2);
    assert_eq!(facts.args[0].source_type_class, "bool");
    assert_eq!(facts.args[1].source_type_class, "uint:u128");
    let range = facts.args[1]
        .scalar_components
        .first()
        .and_then(|scalar| scalar.valid_range.as_ref())
        .ok_or("u128 wrapping range is missing")?;
    assert_eq!(range.start, "340282366920938463463374607431768211455");
    assert_eq!(range.end, "0");
    assert_eq!(facts.ret.source_type_class, "reference");
    Ok(())
}

#[test]
fn root_facts_are_optional_for_legacy_records() -> Result<(), Box<dyn std::error::Error>> {
    let content = r#"{
        "version": 1,
        "compilation_id": "legacy",
        "instances": [{
            "id": "legacy:function",
            "symbol": "function",
            "name": "function",
            "kind": "function",
            "generic_arguments": [],
            "abi": null,
            "abi_error": null,
            "body_status": "absent",
            "body": null,
            "root_obligations": ["root reachability remains unresolved"]
        }]
    }"#;
    let inventory: RawInventory = serde_json::from_str(content)?;
    let instance = inventory
        .instances
        .first()
        .ok_or("legacy record has no instance")?;
    assert!(instance.root_facts.is_none());
    Ok(())
}
