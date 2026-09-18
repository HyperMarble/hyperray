// Layout facts retain scalar components from the compiler layout API.
// They never treat a scalar layout as proof of a valid machine root.

use hyperray_rust::mir::{ScalarFact, WrappingRange};
use rustc_public::abi::{Primitive, Scalar, ValueAbi};
use rustc_public::target::MachineInfo;

pub(super) fn layout_name(layout: &ValueAbi) -> String {
    match layout {
        ValueAbi::Scalar(_) => "scalar".to_string(),
        ValueAbi::ScalarPair { .. } => "scalar_pair".to_string(),
        ValueAbi::Vector { .. } => "vector".to_string(),
        ValueAbi::ScalableVector { .. } => "scalable_vector".to_string(),
        ValueAbi::Aggregate { .. } => "aggregate".to_string(),
    }
}

pub(super) fn components(layout: &ValueAbi) -> Vec<ScalarFact> {
    match layout {
        ValueAbi::Scalar(value) => vec![from_scalar(value)],
        ValueAbi::ScalarPair { a, b, .. } => vec![from_scalar(a), from_scalar(b)],
        ValueAbi::Vector { element, .. } => vec![from_scalar(element)],
        ValueAbi::ScalableVector { element, .. } => vec![from_scalar(element)],
        ValueAbi::Aggregate { .. } => Vec::new(),
    }
}

fn from_scalar(value: &Scalar) -> ScalarFact {
    match value {
        Scalar::Initialized { value, valid_range } => {
            let (primitive, width_bits, signed, address_space) = primitive(*value);
            ScalarFact {
                kind: "initialized".to_string(),
                primitive,
                width_bits,
                signed,
                address_space,
                valid_range: Some(WrappingRange {
                    start: valid_range.start.to_string(),
                    end: valid_range.end.to_string(),
                }),
            }
        }
        Scalar::Union { value } => {
            let (primitive, width_bits, signed, address_space) = primitive(*value);
            ScalarFact {
                kind: "union".to_string(),
                primitive,
                width_bits,
                signed,
                address_space,
                valid_range: None,
            }
        }
    }
}

fn primitive(value: Primitive) -> (String, usize, Option<bool>, Option<u32>) {
    match value {
        Primitive::Int { length, signed } => ("int".to_string(), length.bits(), Some(signed), None),
        Primitive::Float { length } => ("float".to_string(), length.bits(), None, None),
        Primitive::Pointer(address_space) => (
            "pointer".to_string(),
            MachineInfo::target().pointer_width.bits(),
            None,
            Some(address_space.0),
        ),
    }
}
