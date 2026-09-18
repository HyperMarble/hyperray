// A compiler place converted without type text. Every projection variant in
// the pinned compiler has one schema form.

use hyperray_rust::mir::{Place, Projection};
use rustc_public::mir::ProjectionElem;

pub fn of(place: &rustc_public::mir::Place) -> Place {
    Place {
        local: place.local,
        projection: place.projection.iter().map(projection).collect(),
    }
}

fn projection(value: &ProjectionElem) -> Projection {
    match value {
        ProjectionElem::Deref => Projection::Dereference,
        ProjectionElem::Field(index, _) => Projection::Field { index: *index },
        ProjectionElem::Index(local) => Projection::Index { local: *local },
        ProjectionElem::ConstantIndex {
            offset,
            min_length,
            from_end,
        } => Projection::ConstantIndex {
            offset: *offset,
            minimum_length: *min_length,
            from_end: *from_end,
        },
        ProjectionElem::Subslice { from, to, from_end } => Projection::Subslice {
            from: *from,
            to: *to,
            from_end: *from_end,
        },
        ProjectionElem::Downcast(_) => Projection::Downcast,
        ProjectionElem::OpaqueCast(_) => Projection::OpaqueCast,
    }
}
