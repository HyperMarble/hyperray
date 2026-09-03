// The pile a function lands in, from stage3.md Phase A step 4. Order:
// loop, unbuildable, sized, fixed integer, nothing. A loop wins because
// it needs a bound whatever the inputs are.

use super::size::Size;
use serde::Serialize;

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
pub enum Pile {
    NoBound,
    Loop,
    FixedWidth,
    Sized,
    Unbuildable { input: usize, kind: String },
}

pub fn pile(has_loop: bool, inputs: &[Size]) -> Pile {
    if has_loop {
        return Pile::Loop;
    }
    let unbuildable = inputs.iter().enumerate().find_map(|(i, s)| match s {
        Size::Unbuildable(kind) => Some((i, kind.clone())),
        _ => None,
    });
    if let Some((input, kind)) = unbuildable {
        return Pile::Unbuildable { input, kind };
    }
    if inputs.contains(&Size::Sized) {
        return Pile::Sized;
    }
    if inputs.contains(&Size::Fixed) {
        return Pile::FixedWidth;
    }
    Pile::NoBound
}
