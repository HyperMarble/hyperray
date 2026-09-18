// A structural compiler type determines the input proof domain. Printed type
// names must not decide whether an input is finite.

use super::Domain;
use crate::mir::MirType;

pub fn of(ty: &MirType) -> Domain {
    match ty {
        MirType::Scalar { .. } | MirType::Never => Domain::Fixed,
        MirType::Array {
            length: Some(_),
            element,
        } => of(element),
        MirType::Array { length: None, .. } => unknown("array length is not available"),
        MirType::Slice { .. } | MirType::Str => Domain::Dynamic,
        MirType::Reference { target, .. } => of(target),
        MirType::Tuple { fields } => tuple(fields),
        MirType::Adt => unknown("ADT domain has no compiler model"),
        MirType::RawPointer => unknown("raw pointer domain has no compiler model"),
        MirType::Param => unknown("generic parameter needs a concrete value"),
        MirType::Alias => unknown("type alias did not normalize"),
        MirType::Bound => unknown("bound type needs a concrete value"),
        MirType::Callable => unknown("callable input needs a concrete value"),
        MirType::Coroutine => unknown("coroutine input needs a concrete value"),
        MirType::Opaque => unknown("opaque input needs a concrete value"),
    }
}

fn tuple(fields: &[MirType]) -> Domain {
    let domains: Vec<Domain> = fields.iter().map(of).collect();
    match domains
        .iter()
        .find(|domain| matches!(domain, Domain::Unknown { .. }))
    {
        Some(domain) => domain.clone(),
        None if domains.contains(&Domain::Dynamic) => Domain::Dynamic,
        None => Domain::Fixed,
    }
}

fn unknown(reason: &str) -> Domain {
    Domain::Unknown {
        reason: reason.to_string(),
    }
}
