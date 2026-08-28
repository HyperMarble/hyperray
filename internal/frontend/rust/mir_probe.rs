#![allow(dead_code)]

fn choose(flag: bool) -> bool { flag }

#[test]
fn public_relation() {
    assert!(choose(false) != choose(true));
    assert!(choose(true));
}
