use hyperray_rust::bound::{pile, Pile, Size};

// The order is fixed by stage3.md Phase A step 4: loop beats everything,
// then unbuildable, then sized, then fixed, then nothing.
#[test]
fn pile_order_is_loop_unbuildable_sized_fixed_none() {
    let unb = Size::Unbuildable("TypeVar".into());
    assert_eq!(pile(true, &[Size::Fixed]), Pile::Loop);
    assert_eq!(
        pile(false, &[Size::Sized, unb]),
        Pile::Unbuildable {
            input: 1,
            kind: "TypeVar".into()
        }
    );
    assert_eq!(pile(false, &[Size::Fixed, Size::Sized]), Pile::Sized);
    assert_eq!(pile(false, &[Size::Fixed]), Pile::FixedWidth);
    assert_eq!(pile(false, &[]), Pile::NoBound);
}
