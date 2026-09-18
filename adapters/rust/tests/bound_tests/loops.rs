// Compiler cycle diagnostics remain available without bound inference.
// Serialized reports must not expose the removed proposal field.

use crate::bound_case;
use hyperray_rust::bound::analyze;

#[test]
fn a_linear_exit_retains_its_compiler_edges() -> Result<(), hyperray_rust::bound::Error> {
    let result = analyze("crate", &bound_case::counted_loop())?;
    assert_eq!(result.loops.len(), 1);
    assert_eq!(result.loops[0].back_edges.len(), 1);
    assert!(!result.loops[0].exits.is_empty());
    Ok(())
}

#[test]
fn an_interior_comparison_retains_its_cycle() -> Result<(), hyperray_rust::bound::Error> {
    let result = analyze("crate", &bound_case::interior_comparison())?;
    assert_eq!(result.loops.len(), 1);
    Ok(())
}

#[test]
fn compiler_state_edges_do_not_hide_real_cycles() -> Result<(), hyperray_rust::bound::Error> {
    let impossible = analyze("crate", &bound_case::impossible_resume())?;
    let iterator = analyze("crate", &bound_case::iterator_loop())?;
    assert!(impossible.loops.is_empty());
    assert_eq!(iterator.loops.len(), 1);
    assert_eq!(iterator.loops[0].back_edges.len(), 1);
    Ok(())
}

#[test]
fn reports_do_not_contain_inferred_limits() -> Result<(), Box<dyn std::error::Error>> {
    let items = [
        bound_case::counted_loop(),
        bound_case::interior_comparison(),
        bound_case::iterator_loop(),
    ];
    for item in items {
        let result = analyze("crate", &item)?;
        let report = serde_json::to_value(&result.loops[0])?;
        assert!(report.get("proposals").is_none());
        let mut stale = report;
        stale["proposals"] = serde_json::json!([]);
        assert!(serde_json::from_value::<hyperray_rust::bound::Loop>(stale).is_err());
    }
    Ok(())
}
