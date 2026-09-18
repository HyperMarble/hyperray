// Boundary decisions preserve singleton, non-power-of-two, and full-width intervals.
// These arithmetic tests must not claim execution coverage.
use hyperray_rust::prepare::input_bits;

#[test]
fn interval_widths() -> Result<(), Box<dyn std::error::Error>> {
    for (minimum, maximum, expected) in [
        (0, 0, 0),
        (13, 23, 4),
        (0, 65535, 16),
        (u64::MAX - 3, u64::MAX, 2),
        (0, u64::MAX, 64),
    ] {
        assert_eq!(input_bits(minimum, maximum)?, expected);
    }
    assert!(input_bits(2, 1).is_err());
    Ok(())
}

#[test]
fn resource_limits() -> Result<(), Box<dyn std::error::Error>> {
    let original = super::fixture::request()?.limits;
    let mutations: Vec<fn(&mut hyperray_rust::prepare::Limits)> = vec![
        |limits| limits.maximum = 0,
        |limits| limits.search_depth = 0,
        |limits| limits.search_depth = u32::MAX,
        |limits| limits.hash_bits = 0,
        |limits| limits.hash_bits = 64,
        |limits| limits.state_memory_mib = 0,
        |limits| limits.state_memory_mib = u32::MAX,
        |limits| limits.timeout_ms = 0,
        |limits| limits.timeout_ms = u64::MAX,
        |limits| limits.output_limit_bytes = 0,
        |limits| limits.output_limit_bytes = u64::MAX,
        |limits| limits.memory_budget_bytes = 0,
        |limits| limits.memory_budget_bytes = 100000001,
    ];
    for mutation in mutations {
        let mut limits = original.clone();
        mutation(&mut limits);
        assert!(limits.validate().is_err());
    }
    assert_eq!(original.timeout_nanoseconds()?, 10000000000);
    Ok(())
}
