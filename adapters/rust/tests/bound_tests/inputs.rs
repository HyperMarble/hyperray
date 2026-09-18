// Stage 3 keeps each compiler input shape. It must not turn an early type
// classification into a proof limit.

use crate::bound_case;
use hyperray_rust::bound::{analyze, Domain};
use hyperray_rust::mir::{MirType, ScalarKind};

#[test]
fn every_input_keeps_its_compiler_shape() -> Result<(), hyperray_rust::bound::Error> {
    let fixed = bound_case::input(
        0,
        1,
        MirType::Scalar {
            class: ScalarKind::Unsigned,
            bits: 16,
        },
    );
    let dynamic = bound_case::input(
        1,
        2,
        MirType::Slice {
            element: Box::new(MirType::Scalar {
                class: ScalarKind::Unsigned,
                bits: 8,
            }),
        },
    );
    let unknown = bound_case::input(2, 3, MirType::Adt);
    let item = bound_case::item(
        "crate::inputs",
        Some("crate"),
        vec![fixed, dynamic, unknown],
        vec![],
    );
    let result = analyze("crate", &item)?;

    assert_eq!(result.inputs[0].domain, Domain::Fixed);
    assert_eq!(result.inputs[1].domain, Domain::Dynamic);
    assert!(matches!(result.inputs[2].domain, Domain::Unknown { .. }));
    Ok(())
}
