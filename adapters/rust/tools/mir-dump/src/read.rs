// The compiler body used by the dump. Generic items must stay generic because
// forced monomorphization can make rustc normalize an invalid projection.

use rustc_public::mir::Body;
use rustc_public::CrateItem;

pub fn body_of(item: &CrateItem) -> Option<Body> {
    item.body()
}
