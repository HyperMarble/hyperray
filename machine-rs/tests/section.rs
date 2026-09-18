// A relocatable object states its contents in sections. Reading segments
// alone reports such a file as holding nothing.
use hyperray_machine::region;

#[test]
fn an_object_without_segments_reports_its_bytes() {
    for name in ["x86_64.o", "riscv64.o", "arm64.o"] {
        let Ok(bytes) = std::fs::read(format!("tests/fixtures/{name}")) else {
            continue;
        };
        let loaded = region::regions(&bytes)
            .map(|found| found.iter().map(|one| one.bytes.len()).sum::<usize>());
        assert!(matches!(loaded, Ok(count) if count > 0), "{name}: {loaded:?}");
    }
}
