// Charon's body tag mapped to the three states a later stage needs.
// `Missing` is Charon's word for a std body it never had; `Opaque` is one
// it was told to skip. Every variant is named so a new one fails to compile.

use super::seen::Body;
use super::ullbc::BodyTag;

pub fn of(body: &BodyTag) -> Body {
    match body {
        BodyTag::Error(error) => Body::Refused(error.msg.clone()),
        BodyTag::Opaque | BodyTag::Missing => Body::NotRequested,
        BodyTag::Unstructured(_)
        | BodyTag::Structured(_)
        | BodyTag::TargetDispatch(_)
        | BodyTag::Extern(_)
        | BodyTag::Intrinsic(_) => Body::Extracted,
    }
}
